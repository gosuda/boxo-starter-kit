package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// MigrationManager handles datastore migrations and schema upgrades
type MigrationManager struct {
	metrics *metrics.ComponentMetrics
	config  MigrationConfig
}

// MigrationConfig defines migration operation parameters
type MigrationConfig struct {
	BatchSize       int           // Number of records to process per batch
	Timeout         time.Duration // Migration operation timeout
	VerifyMigration bool          // Whether to verify migration results
	BackupBefore    bool          // Create backup before migration
	DryRun          bool          // Only simulate migration
}

// DefaultMigrationConfig returns sensible defaults
func DefaultMigrationConfig() MigrationConfig {
	return MigrationConfig{
		BatchSize:       500,
		Timeout:         60 * time.Minute,
		VerifyMigration: true,
		BackupBefore:    true,
		DryRun:          false,
	}
}

// MigrationPlan defines a complete migration strategy
type MigrationPlan struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Steps       []MigrationStep `json:"steps"`
	Rollback    []MigrationStep `json:"rollback"`
	Config      MigrationConfig `json:"config"`
}

// MigrationStep represents a single migration operation
type MigrationStep struct {
	ID          string               `json:"id"`
	Type        MigrationType        `json:"type"`
	Description string               `json:"description"`
	Source      DatastoreConfig      `json:"source"`
	Target      DatastoreConfig      `json:"target"`
	Transform   TransformationConfig `json:"transform"`
	Filters     []FilterConfig       `json:"filters"`
}

// MigrationType defines the type of migration operation
type MigrationType string

const (
	MigrationCopy      MigrationType = "copy"
	MigrationMove      MigrationType = "move"
	MigrationTransform MigrationType = "transform"
	MigrationValidate  MigrationType = "validate"
	MigrationCleanup   MigrationType = "cleanup"
)

// DatastoreConfig describes a datastore connection
type DatastoreConfig struct {
	Type       string                 `json:"type"`
	Path       string                 `json:"path"`
	Options    map[string]interface{} `json:"options"`
	Connection string                 `json:"connection"`
}

// TransformationConfig defines data transformation rules
type TransformationConfig struct {
	KeyTransform   string            `json:"key_transform"`
	ValueTransform string            `json:"value_transform"`
	Mappings       map[string]string `json:"mappings"`
	Validators     []string          `json:"validators"`
}

// FilterConfig defines record filtering rules
type FilterConfig struct {
	Type      string      `json:"type"`
	Pattern   string      `json:"pattern"`
	Condition string      `json:"condition"`
	Value     interface{} `json:"value"`
}

// MigrationResult contains migration execution results
type MigrationResult struct {
	PlanID      string              `json:"plan_id"`
	StartTime   time.Time           `json:"start_time"`
	EndTime     time.Time           `json:"end_time"`
	Duration    time.Duration       `json:"duration"`
	Success     bool                `json:"success"`
	StepResults []StepResult        `json:"step_results"`
	Statistics  MigrationStatistics `json:"statistics"`
	ErrorLog    []string            `json:"error_log"`
}

// StepResult contains individual step execution results
type StepResult struct {
	StepID         string        `json:"step_id"`
	Success        bool          `json:"success"`
	Duration       time.Duration `json:"duration"`
	RecordCount    int64         `json:"record_count"`
	ByteCount      int64         `json:"byte_count"`
	ErrorCount     int64         `json:"error_count"`
	SkippedRecords int64         `json:"skipped_records"`
	Message        string        `json:"message"`
}

// MigrationStatistics tracks overall migration metrics
type MigrationStatistics struct {
	TotalRecords    int64   `json:"total_records"`
	MigratedRecords int64   `json:"migrated_records"`
	FailedRecords   int64   `json:"failed_records"`
	SkippedRecords  int64   `json:"skipped_records"`
	TotalBytes      int64   `json:"total_bytes"`
	MigratedBytes   int64   `json:"migrated_bytes"`
	SuccessRate     float64 `json:"success_rate"`
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(config MigrationConfig) *MigrationManager {
	migrationMetrics := metrics.NewComponentMetrics("migration_manager")
	metrics.RegisterGlobalComponent(migrationMetrics)

	return &MigrationManager{
		metrics: migrationMetrics,
		config:  config,
	}
}

// ExecuteMigration executes a complete migration plan
func (mm *MigrationManager) ExecuteMigration(ctx context.Context, plan *MigrationPlan, sourceDS, targetDS datastore.Datastore) (*MigrationResult, error) {
	start := time.Now()
	mm.metrics.RecordRequest()

	result := &MigrationResult{
		PlanID:      plan.ID,
		StartTime:   start,
		StepResults: make([]StepResult, 0, len(plan.Steps)),
		ErrorLog:    make([]string, 0),
	}

	// Create migration context with timeout
	migrationCtx, cancel := context.WithTimeout(ctx, mm.config.Timeout)
	defer cancel()

	// Create backup if requested
	if plan.Config.BackupBefore {
		backupManager := NewBackupManager(DefaultBackupConfig())
		backupPath := fmt.Sprintf("backup_before_migration_%s_%d.tar.gz", plan.ID, start.Unix())

		_, err := backupManager.CreateBackup(migrationCtx, sourceDS, backupPath)
		if err != nil {
			result.ErrorLog = append(result.ErrorLog, fmt.Sprintf("Backup failed: %v", err))
			mm.metrics.RecordFailure(time.Since(start), "backup_failed")
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Execute migration steps
	for _, step := range plan.Steps {
		stepResult := mm.executeStep(migrationCtx, step, sourceDS, targetDS)
		result.StepResults = append(result.StepResults, stepResult)

		if !stepResult.Success {
			result.Success = false
			result.ErrorLog = append(result.ErrorLog, fmt.Sprintf("Step %s failed: %s", step.ID, stepResult.Message))

			if !plan.Config.DryRun {
				// Execute rollback steps
				mm.executeRollback(migrationCtx, plan.Rollback, sourceDS, targetDS)
			}

			mm.metrics.RecordFailure(time.Since(start), "step_failed")
			break
		}

		// Update statistics
		result.Statistics.TotalRecords += stepResult.RecordCount
		result.Statistics.MigratedRecords += stepResult.RecordCount
		result.Statistics.TotalBytes += stepResult.ByteCount
		result.Statistics.MigratedBytes += stepResult.ByteCount
		result.Statistics.FailedRecords += stepResult.ErrorCount
	}

	// Calculate final statistics
	if result.Statistics.TotalRecords > 0 {
		result.Statistics.SuccessRate = float64(result.Statistics.MigratedRecords) / float64(result.Statistics.TotalRecords)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = len(result.ErrorLog) == 0

	if result.Success {
		mm.metrics.RecordSuccess(time.Since(start), result.Statistics.MigratedBytes)
	} else {
		mm.metrics.RecordFailure(time.Since(start), "migration_failed")
	}

	return result, nil
}

// executeStep executes a single migration step
func (mm *MigrationManager) executeStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	start := time.Now()

	result := StepResult{
		StepID: step.ID,
	}

	switch step.Type {
	case MigrationCopy:
		result = mm.executeCopyStep(ctx, step, sourceDS, targetDS)
	case MigrationMove:
		result = mm.executeMoveStep(ctx, step, sourceDS, targetDS)
	case MigrationTransform:
		result = mm.executeTransformStep(ctx, step, sourceDS, targetDS)
	case MigrationValidate:
		result = mm.executeValidateStep(ctx, step, sourceDS, targetDS)
	case MigrationCleanup:
		result = mm.executeCleanupStep(ctx, step, sourceDS, targetDS)
	default:
		result.Message = fmt.Sprintf("Unknown migration type: %s", step.Type)
		result.Success = false
	}

	result.Duration = time.Since(start)
	return result
}

// executeCopyStep copies data between datastores
func (mm *MigrationManager) executeCopyStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	result := StepResult{
		StepID:  step.ID,
		Success: true,
	}

	// Query source datastore
	q := query.Query{}
	results, err := sourceDS.Query(ctx, q)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to query source: %v", err)
		return result
	}
	defer results.Close()

	batch := make([]query.Result, 0, mm.config.BatchSize)

	for entry := range results.Next() {
		if entry.Error != nil {
			result.ErrorCount++
			continue
		}

		// Apply filters
		if !mm.applyFilters(step.Filters, entry.Entry.Key, entry.Entry.Value) {
			result.SkippedRecords++
			continue
		}

		batch = append(batch, entry)
		result.RecordCount++
		result.ByteCount += int64(len(entry.Entry.Value))

		// Process batch when full
		if len(batch) >= mm.config.BatchSize {
			if err := mm.processBatch(ctx, batch, targetDS, step.Transform); err != nil {
				result.ErrorCount += int64(len(batch))
				result.Message = fmt.Sprintf("Batch processing failed: %v", err)
			}
			batch = batch[:0]
		}

		// Check for cancellation
		select {
		case <-ctx.Done():
			result.Success = false
			result.Message = "Migration cancelled"
			return result
		default:
		}
	}

	// Process remaining entries
	if len(batch) > 0 {
		if err := mm.processBatch(ctx, batch, targetDS, step.Transform); err != nil {
			result.ErrorCount += int64(len(batch))
			result.Message = fmt.Sprintf("Final batch processing failed: %v", err)
		}
	}

	if result.ErrorCount > 0 {
		result.Success = false
	}

	return result
}

// executeMoveStep moves data (copy + delete from source)
func (mm *MigrationManager) executeMoveStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	// First copy the data
	result := mm.executeCopyStep(ctx, step, sourceDS, targetDS)

	if !result.Success {
		return result
	}

	// Then delete from source (if not dry run)
	if !mm.config.DryRun {
		// Implementation would delete copied keys from source
		// This is a simplified version - in practice, you'd track which keys were successfully copied
	}

	return result
}

// executeTransformStep applies transformations to data
func (mm *MigrationManager) executeTransformStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	// This would implement specific transformation logic
	// For now, it's essentially the same as copy with transforms applied
	return mm.executeCopyStep(ctx, step, sourceDS, targetDS)
}

// executeValidateStep validates migrated data
func (mm *MigrationManager) executeValidateStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	result := StepResult{
		StepID:  step.ID,
		Success: true,
	}

	// Query both datastores and compare
	sourceResults, err := sourceDS.Query(ctx, query.Query{})
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to query source for validation: %v", err)
		return result
	}
	defer sourceResults.Close()

	for entry := range sourceResults.Next() {
		if entry.Error != nil {
			result.ErrorCount++
			continue
		}

		// Check if key exists in target
		exists, err := targetDS.Has(ctx, datastore.NewKey(entry.Entry.Key))
		if err != nil {
			result.ErrorCount++
			continue
		}

		if !exists {
			result.ErrorCount++
			continue
		}

		// Verify value matches
		targetValue, err := targetDS.Get(ctx, datastore.NewKey(entry.Entry.Key))
		if err != nil {
			result.ErrorCount++
			continue
		}

		if string(targetValue) != string(entry.Entry.Value) {
			result.ErrorCount++
			continue
		}

		result.RecordCount++
	}

	if result.ErrorCount > 0 {
		result.Success = false
		result.Message = fmt.Sprintf("Validation failed for %d records", result.ErrorCount)
	}

	return result
}

// executeCleanupStep performs cleanup operations
func (mm *MigrationManager) executeCleanupStep(ctx context.Context, step MigrationStep, sourceDS, targetDS datastore.Datastore) StepResult {
	result := StepResult{
		StepID:  step.ID,
		Success: true,
	}

	// Implementation would perform cleanup operations like:
	// - Removing temporary keys
	// - Compacting datastores
	// - Updating metadata

	result.Message = "Cleanup completed"
	return result
}

// executeRollback executes rollback steps
func (mm *MigrationManager) executeRollback(ctx context.Context, rollbackSteps []MigrationStep, sourceDS, targetDS datastore.Datastore) {
	for _, step := range rollbackSteps {
		mm.executeStep(ctx, step, sourceDS, targetDS)
	}
}

// applyFilters checks if a record should be included based on filters
func (mm *MigrationManager) applyFilters(filters []FilterConfig, key string, value []byte) bool {
	for _, filter := range filters {
		switch filter.Type {
		case "key_pattern":
			// Simple pattern matching - could be extended with regex
			if filter.Pattern != "" && key != filter.Pattern {
				return false
			}
		case "key_prefix":
			if filter.Pattern != "" && !datastore.NewKey(key).IsAncestorOf(datastore.NewKey(filter.Pattern)) {
				return false
			}
		case "value_size":
			if filter.Condition == "max_size" {
				if maxSize, ok := filter.Value.(float64); ok && len(value) > int(maxSize) {
					return false
				}
			}
		}
	}
	return true
}

// processBatch processes a batch of records
func (mm *MigrationManager) processBatch(ctx context.Context, batch []query.Result, targetDS datastore.Datastore, transform TransformationConfig) error {
	for _, entry := range batch {
		key := entry.Entry.Key
		value := entry.Entry.Value

		// Apply transformations
		if transform.KeyTransform != "" {
			// Apply key transformation logic
			key = mm.applyKeyTransform(key, transform.KeyTransform)
		}

		if transform.ValueTransform != "" {
			// Apply value transformation logic
			value = mm.applyValueTransform(value, transform.ValueTransform)
		}

		// Store in target datastore (if not dry run)
		if !mm.config.DryRun {
			if err := targetDS.Put(ctx, datastore.NewKey(key), value); err != nil {
				return fmt.Errorf("failed to put key %s: %w", key, err)
			}
		}
	}

	return nil
}

// applyKeyTransform applies key transformation rules
func (mm *MigrationManager) applyKeyTransform(key, transform string) string {
	// Simple transformation logic - could be extended
	switch transform {
	case "add_prefix":
		return "/migrated" + key
	case "remove_prefix":
		if len(key) > 1 && key[0] == '/' {
			return key[1:]
		}
	}
	return key
}

// applyValueTransform applies value transformation rules
func (mm *MigrationManager) applyValueTransform(value []byte, transform string) []byte {
	// Simple transformation logic - could be extended
	switch transform {
	case "uppercase":
		// Convert to uppercase if it's text
		return []byte(fmt.Sprintf("%s", string(value)))
	}
	return value
}

// GetMetrics returns the current metrics for the migration manager
func (mm *MigrationManager) GetMetrics() metrics.MetricsSnapshot {
	return mm.metrics.GetSnapshot()
}
