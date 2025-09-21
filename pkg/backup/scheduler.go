package backup

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// BackupScheduler manages automatic backup scheduling and execution
type BackupScheduler struct {
	metrics      *metrics.ComponentMetrics
	config       SchedulerConfig
	backupManager *BackupManager

	mu        sync.RWMutex
	schedules map[string]*ScheduledBackup
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// SchedulerConfig defines scheduler parameters
type SchedulerConfig struct {
	DefaultBackupDir   string        // Default directory for backups
	RetentionPolicy    RetentionPolicy // How long to keep backups
	ConcurrentBackups  int           // Maximum concurrent backup operations
	HealthCheckInterval time.Duration // How often to check backup health
	NotificationConfig NotificationConfig // Alert settings
}

// RetentionPolicy defines backup retention rules
type RetentionPolicy struct {
	KeepDaily   int // Number of daily backups to keep
	KeepWeekly  int // Number of weekly backups to keep
	KeepMonthly int // Number of monthly backups to keep
	KeepYearly  int // Number of yearly backups to keep
	MaxAge      time.Duration // Maximum age for any backup
}

// NotificationConfig defines alerting settings
type NotificationConfig struct {
	EmailOnFailure bool     // Send email on backup failure
	EmailOnSuccess bool     // Send email on backup success
	Recipients     []string // Email recipients
	WebhookURL     string   // Webhook for notifications
}

// ScheduledBackup represents a scheduled backup job
type ScheduledBackup struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Schedule    string          `json:"schedule"` // Cron expression
	Datastore   datastore.Datastore `json:"-"`
	Config      BackupConfig    `json:"config"`
	Enabled     bool            `json:"enabled"`
	LastRun     time.Time       `json:"last_run"`
	NextRun     time.Time       `json:"next_run"`
	LastResult  *BackupResult   `json:"last_result"`
	Statistics  BackupJobStats  `json:"statistics"`

	// Internal fields
	cronSchedule *cronSchedule
	ticker       *time.Ticker
}

// BackupResult contains the result of a backup operation
type BackupResult struct {
	Success    bool          `json:"success"`
	StartTime  time.Time     `json:"start_time"`
	Duration   time.Duration `json:"duration"`
	FilePath   string        `json:"file_path"`
	FileSize   int64         `json:"file_size"`
	KeyCount   int64         `json:"key_count"`
	ErrorMsg   string        `json:"error_msg"`
	Metadata   *BackupMetadata `json:"metadata"`
}

// BackupJobStats tracks statistics for a backup job
type BackupJobStats struct {
	TotalRuns      int64         `json:"total_runs"`
	SuccessfulRuns int64         `json:"successful_runs"`
	FailedRuns     int64         `json:"failed_runs"`
	AverageDuration time.Duration `json:"average_duration"`
	LastSuccess    time.Time     `json:"last_success"`
	LastFailure    time.Time     `json:"last_failure"`
	SuccessRate    float64       `json:"success_rate"`
}

// cronSchedule represents a cron-like schedule
type cronSchedule struct {
	expression string
	interval   time.Duration
}

// DefaultSchedulerConfig returns sensible defaults
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		DefaultBackupDir:   "./backups",
		RetentionPolicy: RetentionPolicy{
			KeepDaily:   7,
			KeepWeekly:  4,
			KeepMonthly: 12,
			KeepYearly:  5,
			MaxAge:      365 * 24 * time.Hour, // 1 year
		},
		ConcurrentBackups:   2,
		HealthCheckInterval: 1 * time.Hour,
	}
}

// NewBackupScheduler creates a new backup scheduler
func NewBackupScheduler(config SchedulerConfig) *BackupScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	schedulerMetrics := metrics.NewComponentMetrics("backup_scheduler")
	metrics.RegisterGlobalComponent(schedulerMetrics)

	return &BackupScheduler{
		metrics:       schedulerMetrics,
		config:        config,
		backupManager: NewBackupManager(DefaultBackupConfig()),
		schedules:     make(map[string]*ScheduledBackup),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the backup scheduler
func (bs *BackupScheduler) Start() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.running {
		return fmt.Errorf("scheduler already running")
	}

	bs.running = true

	// Start scheduler worker
	bs.wg.Add(1)
	go bs.schedulerWorker()

	// Start health checker
	bs.wg.Add(1)
	go bs.healthChecker()

	return nil
}

// Stop stops the backup scheduler
func (bs *BackupScheduler) Stop() error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.running {
		return fmt.Errorf("scheduler not running")
	}

	bs.running = false
	bs.cancel()
	bs.wg.Wait()

	return nil
}

// AddSchedule adds a new scheduled backup
func (bs *BackupScheduler) AddSchedule(schedule *ScheduledBackup) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if _, exists := bs.schedules[schedule.ID]; exists {
		return fmt.Errorf("schedule with ID %s already exists", schedule.ID)
	}

	// Parse cron schedule
	cronSched, err := bs.parseCronSchedule(schedule.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule format: %w", err)
	}

	schedule.cronSchedule = cronSched
	schedule.NextRun = bs.calculateNextRun(cronSched)

	bs.schedules[schedule.ID] = schedule
	return nil
}

// RemoveSchedule removes a scheduled backup
func (bs *BackupScheduler) RemoveSchedule(scheduleID string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if _, exists := bs.schedules[scheduleID]; !exists {
		return fmt.Errorf("schedule with ID %s not found", scheduleID)
	}

	delete(bs.schedules, scheduleID)
	return nil
}

// GetSchedule returns a scheduled backup by ID
func (bs *BackupScheduler) GetSchedule(scheduleID string) (*ScheduledBackup, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	schedule, exists := bs.schedules[scheduleID]
	if !exists {
		return nil, fmt.Errorf("schedule with ID %s not found", scheduleID)
	}

	return schedule, nil
}

// ListSchedules returns all scheduled backups
func (bs *BackupScheduler) ListSchedules() []*ScheduledBackup {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	schedules := make([]*ScheduledBackup, 0, len(bs.schedules))
	for _, schedule := range bs.schedules {
		schedules = append(schedules, schedule)
	}

	return schedules
}

// ExecuteBackup manually executes a backup
func (bs *BackupScheduler) ExecuteBackup(scheduleID string) (*BackupResult, error) {
	start := time.Now()
	bs.metrics.RecordRequest()

	schedule, err := bs.GetSchedule(scheduleID)
	if err != nil {
		bs.metrics.RecordFailure(time.Since(start), "schedule_not_found")
		return nil, err
	}

	return bs.executeScheduledBackup(schedule)
}

// schedulerWorker runs the main scheduling loop
func (bs *BackupScheduler) schedulerWorker() {
	defer bs.wg.Done()

	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bs.checkSchedules()
		case <-bs.ctx.Done():
			return
		}
	}
}

// checkSchedules checks if any backups need to be executed
func (bs *BackupScheduler) checkSchedules() {
	bs.mu.RLock()
	now := time.Now()
	toExecute := make([]*ScheduledBackup, 0)

	for _, schedule := range bs.schedules {
		if schedule.Enabled && now.After(schedule.NextRun) {
			toExecute = append(toExecute, schedule)
		}
	}
	bs.mu.RUnlock()

	// Execute due backups
	for _, schedule := range toExecute {
		go func(s *ScheduledBackup) {
			result, err := bs.executeScheduledBackup(s)
			if err != nil {
				log.Printf("Failed to execute backup %s: %v", s.ID, err)
			} else {
				bs.updateScheduleResult(s.ID, result)
			}
		}(schedule)
	}
}

// executeScheduledBackup executes a single scheduled backup
func (bs *BackupScheduler) executeScheduledBackup(schedule *ScheduledBackup) (*BackupResult, error) {
	start := time.Now()

	result := &BackupResult{
		StartTime: start,
	}

	// Generate backup filename
	timestamp := start.Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.tar.gz", schedule.Name, timestamp)
	filePath := filepath.Join(bs.config.DefaultBackupDir, filename)

	// Execute backup
	metadata, err := bs.backupManager.CreateBackup(bs.ctx, schedule.Datastore, filePath)
	if err != nil {
		result.Success = false
		result.ErrorMsg = err.Error()
		result.Duration = time.Since(start)
		return result, err
	}

	// Get file size
	if fileInfo, err := filepath.Glob(filePath); err == nil && len(fileInfo) > 0 {
		if stat, err := filepath.EvalSymlinks(filePath); err == nil {
			result.FileSize = int64(len(stat))
		}
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.FilePath = filePath
	result.KeyCount = metadata.TotalKeys
	result.Metadata = metadata

	// Update schedule
	bs.mu.Lock()
	schedule.LastRun = start
	schedule.NextRun = bs.calculateNextRun(schedule.cronSchedule)
	schedule.LastResult = result
	bs.updateJobStatistics(schedule, result)
	bs.mu.Unlock()

	// Send notifications if configured
	bs.sendNotification(schedule, result)

	return result, nil
}

// updateScheduleResult updates the result for a schedule
func (bs *BackupScheduler) updateScheduleResult(scheduleID string, result *BackupResult) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if schedule, exists := bs.schedules[scheduleID]; exists {
		schedule.LastResult = result
		bs.updateJobStatistics(schedule, result)
	}
}

// updateJobStatistics updates statistics for a backup job
func (bs *BackupScheduler) updateJobStatistics(schedule *ScheduledBackup, result *BackupResult) {
	stats := &schedule.Statistics
	stats.TotalRuns++

	if result.Success {
		stats.SuccessfulRuns++
		stats.LastSuccess = result.StartTime
	} else {
		stats.FailedRuns++
		stats.LastFailure = result.StartTime
	}

	// Update average duration
	if stats.TotalRuns > 1 {
		totalDuration := stats.AverageDuration*time.Duration(stats.TotalRuns-1) + result.Duration
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalRuns)
	} else {
		stats.AverageDuration = result.Duration
	}

	// Calculate success rate
	if stats.TotalRuns > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalRuns)
	}
}

// healthChecker periodically checks backup health and cleans up old backups
func (bs *BackupScheduler) healthChecker() {
	defer bs.wg.Done()

	ticker := time.NewTicker(bs.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bs.performHealthCheck()
			bs.cleanupOldBackups()
		case <-bs.ctx.Done():
			return
		}
	}
}

// performHealthCheck checks the health of recent backups
func (bs *BackupScheduler) performHealthCheck() {
	bs.mu.RLock()
	schedules := make([]*ScheduledBackup, 0, len(bs.schedules))
	for _, schedule := range bs.schedules {
		schedules = append(schedules, schedule)
	}
	bs.mu.RUnlock()

	for _, schedule := range schedules {
		if schedule.LastResult != nil && schedule.LastResult.Success {
			// Verify backup file still exists and is readable
			if schedule.LastResult.FilePath != "" {
				_, err := bs.backupManager.VerifyBackup(bs.ctx, schedule.LastResult.FilePath)
				if err != nil {
					log.Printf("Health check failed for backup %s: %v", schedule.ID, err)
				}
			}
		}
	}
}

// cleanupOldBackups removes old backups according to retention policy
func (bs *BackupScheduler) cleanupOldBackups() {
	// Implementation would:
	// 1. List all backup files in backup directory
	// 2. Group by backup job
	// 3. Apply retention policy to each group
	// 4. Delete files that exceed retention limits

	// This is a simplified placeholder
	log.Println("Cleanup of old backups (placeholder implementation)")
}

// sendNotification sends notifications based on backup results
func (bs *BackupScheduler) sendNotification(schedule *ScheduledBackup, result *BackupResult) {
	if bs.config.NotificationConfig.EmailOnFailure && !result.Success {
		bs.sendEmailNotification(schedule, result, "FAILURE")
	}

	if bs.config.NotificationConfig.EmailOnSuccess && result.Success {
		bs.sendEmailNotification(schedule, result, "SUCCESS")
	}

	if bs.config.NotificationConfig.WebhookURL != "" {
		bs.sendWebhookNotification(schedule, result)
	}
}

// sendEmailNotification sends email notification
func (bs *BackupScheduler) sendEmailNotification(schedule *ScheduledBackup, result *BackupResult, status string) {
	// Placeholder for email notification implementation
	log.Printf("Email notification for backup %s: %s", schedule.ID, status)
}

// sendWebhookNotification sends webhook notification
func (bs *BackupScheduler) sendWebhookNotification(schedule *ScheduledBackup, result *BackupResult) {
	// Placeholder for webhook notification implementation
	log.Printf("Webhook notification for backup %s", schedule.ID)
}

// parseCronSchedule parses a cron expression into a schedule
func (bs *BackupScheduler) parseCronSchedule(expression string) (*cronSchedule, error) {
	// Simplified cron parsing - in production, use a proper cron library
	switch expression {
	case "@daily", "0 0 * * *":
		return &cronSchedule{expression: expression, interval: 24 * time.Hour}, nil
	case "@hourly", "0 * * * *":
		return &cronSchedule{expression: expression, interval: time.Hour}, nil
	case "@weekly", "0 0 * * 0":
		return &cronSchedule{expression: expression, interval: 7 * 24 * time.Hour}, nil
	default:
		return nil, fmt.Errorf("unsupported cron expression: %s", expression)
	}
}

// calculateNextRun calculates the next run time for a schedule
func (bs *BackupScheduler) calculateNextRun(schedule *cronSchedule) time.Time {
	return time.Now().Add(schedule.interval)
}

// GetMetrics returns the current metrics for the backup scheduler
func (bs *BackupScheduler) GetMetrics() metrics.MetricsSnapshot {
	return bs.metrics.GetSnapshot()
}