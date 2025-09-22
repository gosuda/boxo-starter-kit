package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
)

func TestBackupManager_CreateBackup(t *testing.T) {
	// Create test datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Add test data
	testData := map[string][]byte{
		"/blocks/test1": []byte("test data 1"),
		"/blocks/test2": []byte("test data 2"),
		"/local/config": []byte("config data"),
	}

	ctx := context.Background()
	for key, value := range testData {
		err := ds.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create backup manager
	config := DefaultBackupConfig()
	config.ChunkSize = 2 // Small chunk size for testing
	manager := NewBackupManager(config)

	// Create temporary backup file
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test-backup.tar.gz")

	// Create backup
	metadata, err := manager.CreateBackup(ctx, ds, backupPath)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Verify metadata
	if metadata.TotalKeys != int64(len(testData)) {
		t.Errorf("Expected %d keys, got %d", len(testData), metadata.TotalKeys)
	}

	if metadata.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", metadata.Version)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file not created")
	}
}

func TestBackupManager_RestoreBackup(t *testing.T) {
	ctx := context.Background()

	// Create source datastore with test data
	sourceDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer sourceDS.Close()

	testData := map[string][]byte{
		"/blocks/test1": []byte("test data 1"),
		"/blocks/test2": []byte("test data 2"),
		"/local/config": []byte("config data"),
	}

	for key, value := range testData {
		err := sourceDS.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create backup
	manager := NewBackupManager(DefaultBackupConfig())
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test-backup.tar.gz")

	_, err := manager.CreateBackup(ctx, sourceDS, backupPath)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Create target datastore
	targetDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer targetDS.Close()

	// Restore backup
	metadata, err := manager.RestoreBackup(ctx, backupPath, targetDS)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Verify metadata
	if metadata.TotalKeys != int64(len(testData)) {
		t.Errorf("Expected %d keys, got %d", len(testData), metadata.TotalKeys)
	}

	// Verify all data was restored
	for key, expectedValue := range testData {
		value, err := targetDS.Get(ctx, datastore.NewKey(key))
		if err != nil {
			t.Errorf("Failed to get key %s: %v", key, err)
			continue
		}
		if string(value) != string(expectedValue) {
			t.Errorf("Data mismatch for key %s: expected %s, got %s", key, expectedValue, value)
		}
	}
}

func TestBackupManager_VerifyBackup(t *testing.T) {
	ctx := context.Background()

	// Create test datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Add test data
	testData := map[string][]byte{
		"/blocks/test1": []byte("test data 1"),
		"/blocks/test2": []byte("test data 2"),
	}

	for key, value := range testData {
		err := ds.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create backup
	manager := NewBackupManager(DefaultBackupConfig())
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test-backup.tar.gz")

	_, err := manager.CreateBackup(ctx, ds, backupPath)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Verify backup
	metadata, err := manager.VerifyBackup(ctx, backupPath)
	if err != nil {
		t.Fatalf("VerifyBackup failed: %v", err)
	}

	// Check metadata
	if metadata.TotalKeys != int64(len(testData)) {
		t.Errorf("Expected %d keys, got %d", len(testData), metadata.TotalKeys)
	}
}

func TestBackupManager_ExcludePatterns(t *testing.T) {
	ctx := context.Background()

	// Create test datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Add test data with patterns to exclude
	testData := map[string][]byte{
		"/blocks/test1":    []byte("include this"),
		"/blocks/test2":    []byte("include this too"),
		"/temp/temporary":  []byte("exclude this"),
		"/cache/cached":    []byte("exclude this too"),
		"/local/important": []byte("include this"),
	}

	for key, value := range testData {
		err := ds.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create backup config with exclude patterns
	config := DefaultBackupConfig()
	config.ExcludePatterns = []string{"/temp/*", "/cache/*"}
	manager := NewBackupManager(config)

	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test-backup.tar.gz")

	// Create backup
	metadata, err := manager.CreateBackup(ctx, ds, backupPath)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Should have excluded 2 keys (/temp and /cache)
	expectedKeys := int64(3) // /blocks/test1, /blocks/test2, /local/important
	if metadata.TotalKeys != expectedKeys {
		t.Errorf("Expected %d keys after exclusion, got %d", expectedKeys, metadata.TotalKeys)
	}

	// Restore to verify exclusion worked
	targetDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer targetDS.Close()

	_, err = manager.RestoreBackup(ctx, backupPath, targetDS)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Check that excluded keys are not present
	excludedKeys := []string{"/temp/temporary", "/cache/cached"}
	for _, key := range excludedKeys {
		_, err := targetDS.Get(ctx, datastore.NewKey(key))
		if err != datastore.ErrNotFound {
			t.Errorf("Excluded key %s should not be present", key)
		}
	}

	// Check that included keys are present
	includedKeys := []string{"/blocks/test1", "/blocks/test2", "/local/important"}
	for _, key := range includedKeys {
		_, err := targetDS.Get(ctx, datastore.NewKey(key))
		if err != nil {
			t.Errorf("Included key %s should be present: %v", key, err)
		}
	}
}

func TestMigrationManager_ExecuteMigration(t *testing.T) {
	ctx := context.Background()

	// Create source datastore with test data
	sourceDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer sourceDS.Close()

	testData := map[string][]byte{
		"/blocks/test1": []byte("test data 1"),
		"/blocks/test2": []byte("test data 2"),
	}

	for key, value := range testData {
		err := sourceDS.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create target datastore
	targetDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer targetDS.Close()

	// Create migration plan
	plan := &MigrationPlan{
		ID:          "test-migration",
		Version:     "1.0",
		Description: "Test migration",
		Steps: []MigrationStep{
			{
				ID:          "copy-data",
				Type:        MigrationCopy,
				Description: "Copy all data",
			},
			{
				ID:          "validate-data",
				Type:        MigrationValidate,
				Description: "Validate copied data",
			},
		},
		Config: DefaultMigrationConfig(),
	}

	// Execute migration
	manager := NewMigrationManager(plan.Config)
	result, err := manager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
	if err != nil {
		t.Fatalf("ExecuteMigration failed: %v", err)
	}

	// Check result
	if !result.Success {
		t.Errorf("Migration should have succeeded")
		for _, errMsg := range result.ErrorLog {
			t.Logf("Error: %s", errMsg)
		}
	}

	if len(result.StepResults) != len(plan.Steps) {
		t.Errorf("Expected %d step results, got %d", len(plan.Steps), len(result.StepResults))
	}

	// Verify data was copied
	for key, expectedValue := range testData {
		value, err := targetDS.Get(ctx, datastore.NewKey(key))
		if err != nil {
			t.Errorf("Failed to get migrated key %s: %v", key, err)
			continue
		}
		if string(value) != string(expectedValue) {
			t.Errorf("Data mismatch for key %s: expected %s, got %s", key, expectedValue, value)
		}
	}
}

func TestMigrationManager_DryRun(t *testing.T) {
	ctx := context.Background()

	// Create source datastore with test data
	sourceDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer sourceDS.Close()

	testData := map[string][]byte{
		"/blocks/test1": []byte("test data 1"),
	}

	for key, value := range testData {
		err := sourceDS.Put(ctx, datastore.NewKey(key), value)
		if err != nil {
			t.Fatalf("Failed to put test data: %v", err)
		}
	}

	// Create target datastore
	targetDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer targetDS.Close()

	// Create migration plan with dry run
	config := DefaultMigrationConfig()
	config.DryRun = true

	plan := &MigrationPlan{
		ID:      "test-dry-run",
		Version: "1.0",
		Steps: []MigrationStep{
			{
				ID:   "copy-data",
				Type: MigrationCopy,
			},
		},
		Config: config,
	}

	// Execute dry run
	manager := NewMigrationManager(config)
	result, err := manager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
	if err != nil {
		t.Fatalf("ExecuteMigration failed: %v", err)
	}

	// Should succeed but not actually copy data
	if !result.Success {
		t.Errorf("Dry run should have succeeded")
	}

	// Target datastore should still be empty
	has, err := targetDS.Has(ctx, datastore.NewKey("/blocks/test1"))
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}
	if has {
		t.Errorf("Dry run should not have copied data")
	}
}

func TestBackupScheduler_AddRemoveSchedule(t *testing.T) {
	scheduler := NewBackupScheduler(DefaultSchedulerConfig())

	// Create test datastore
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Create test schedule
	schedule := &ScheduledBackup{
		ID:        "test-schedule",
		Name:      "test",
		Schedule:  "@daily",
		Datastore: ds,
		Enabled:   true,
	}

	// Add schedule
	err := scheduler.AddSchedule(schedule)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}

	// Verify schedule was added
	retrieved, err := scheduler.GetSchedule("test-schedule")
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}

	if retrieved.ID != schedule.ID {
		t.Errorf("Expected schedule ID %s, got %s", schedule.ID, retrieved.ID)
	}

	// List schedules
	schedules := scheduler.ListSchedules()
	if len(schedules) != 1 {
		t.Errorf("Expected 1 schedule, got %d", len(schedules))
	}

	// Remove schedule
	err = scheduler.RemoveSchedule("test-schedule")
	if err != nil {
		t.Fatalf("RemoveSchedule failed: %v", err)
	}

	// Verify schedule was removed
	_, err = scheduler.GetSchedule("test-schedule")
	if err == nil {
		t.Errorf("Schedule should have been removed")
	}
}

func TestBackupScheduler_StartStop(t *testing.T) {
	scheduler := NewBackupScheduler(DefaultSchedulerConfig())

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Starting again should fail
	err = scheduler.Start()
	if err == nil {
		t.Errorf("Starting scheduler twice should fail")
	}

	// Stop scheduler
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Stopping again should fail
	err = scheduler.Stop()
	if err == nil {
		t.Errorf("Stopping scheduler twice should fail")
	}
}

func TestBackupMetrics(t *testing.T) {
	manager := NewBackupManager(DefaultBackupConfig())

	// Get initial metrics
	metrics := manager.GetMetrics()
	if metrics.ComponentName != "backup_manager" {
		t.Errorf("Expected component name 'backup_manager', got %s", metrics.ComponentName)
	}

	initialRequests := metrics.TotalRequests

	// Perform an operation to generate metrics
	ctx := context.Background()
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Add some test data
	err := ds.Put(ctx, datastore.NewKey("/test"), []byte("test"))
	if err != nil {
		t.Fatalf("Failed to put test data: %v", err)
	}

	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "metrics-test.tar.gz")

	_, err = manager.CreateBackup(ctx, ds, backupPath)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Check metrics were updated
	updatedMetrics := manager.GetMetrics()
	if updatedMetrics.TotalRequests <= initialRequests {
		t.Errorf("Metrics should have been updated")
	}
}

// Benchmark tests
func BenchmarkBackupManager_CreateBackup(b *testing.B) {
	ctx := context.Background()
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	defer ds.Close()

	// Add test data
	for i := 0; i < 1000; i++ {
		key := datastore.NewKey(fmt.Sprintf("/blocks/test%d", i))
		value := []byte(fmt.Sprintf("test data %d", i))
		ds.Put(ctx, key, value)
	}

	manager := NewBackupManager(DefaultBackupConfig())
	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backupPath := filepath.Join(tempDir, fmt.Sprintf("benchmark-%d.tar.gz", i))
		_, err := manager.CreateBackup(ctx, ds, backupPath)
		if err != nil {
			b.Fatalf("CreateBackup failed: %v", err)
		}
	}
}

func BenchmarkMigrationManager_ExecuteMigration(b *testing.B) {
	ctx := context.Background()

	// Setup
	sourceDS := sync.MutexWrap(datastore.NewMapDatastore())
	defer sourceDS.Close()

	// Add test data
	for i := 0; i < 100; i++ {
		key := datastore.NewKey(fmt.Sprintf("/blocks/test%d", i))
		value := []byte(fmt.Sprintf("test data %d", i))
		sourceDS.Put(ctx, key, value)
	}

	plan := &MigrationPlan{
		ID:      "benchmark-migration",
		Version: "1.0",
		Steps: []MigrationStep{
			{
				ID:   "copy-data",
				Type: MigrationCopy,
			},
		},
		Config: DefaultMigrationConfig(),
	}

	manager := NewMigrationManager(plan.Config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		targetDS := sync.MutexWrap(datastore.NewMapDatastore())
		_, err := manager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
		if err != nil {
			b.Fatalf("ExecuteMigration failed: %v", err)
		}
		targetDS.Close()
	}
}
