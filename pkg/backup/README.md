# Backup and Migration Tools

This package provides comprehensive backup, restore, and migration capabilities for IPFS datastores in the boxo ecosystem.

## Features

### 🔄 Backup Management
- **Compressed Backups**: Gzip compression with configurable levels
- **Incremental Processing**: Chunked processing for large datastores
- **Integrity Verification**: Built-in backup verification and checksums
- **Metadata Tracking**: Complete backup metadata and statistics
- **Flexible Filtering**: Exclude patterns and custom filters

### 🚀 Migration System
- **Multi-Step Migrations**: Complex migration plans with rollback support
- **Data Transformation**: Key/value transformation rules
- **Validation**: Post-migration data integrity validation
- **Dry Run Mode**: Test migrations without making changes
- **Batch Processing**: Efficient processing of large datasets

### ⏰ Automated Scheduling
- **Cron-like Scheduling**: Automatic backup scheduling
- **Retention Policies**: Configurable backup retention rules
- **Health Monitoring**: Continuous backup health checking
- **Notifications**: Email and webhook notifications
- **Concurrent Operations**: Multiple simultaneous backup jobs

## Core Components

### BackupManager

Handles creation, restoration, and verification of datastore backups.

```go
// Create a backup manager
config := backup.DefaultBackupConfig()
config.CompressionLevel = 9  // Maximum compression
config.ChunkSize = 1000      // Process 1000 records per chunk

manager := backup.NewBackupManager(config)

// Create a backup
metadata, err := manager.CreateBackup(ctx, datastore, "backup.tar.gz")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Backup created: %d keys, %d bytes\n",
    metadata.TotalKeys, metadata.TotalSize)
```

### MigrationManager

Handles complex datastore migrations with transformation support.

```go
// Create a migration plan
plan := &backup.MigrationPlan{
    ID:          "migrate-v1-to-v2",
    Version:     "2.0",
    Description: "Migrate datastore schema v1 to v2",
    Steps: []backup.MigrationStep{
        {
            ID:   "copy-blocks",
            Type: backup.MigrationCopy,
            Transform: backup.TransformationConfig{
                KeyTransform: "add_prefix",
            },
        },
        {
            ID:   "validate-migration",
            Type: backup.MigrationValidate,
        },
    },
}

// Execute migration
migrationManager := backup.NewMigrationManager(backup.DefaultMigrationConfig())
result, err := migrationManager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
```

### BackupScheduler

Provides automated backup scheduling and management.

```go
// Create scheduler
scheduler := backup.NewBackupScheduler(backup.DefaultSchedulerConfig())

// Add a scheduled backup
schedule := &backup.ScheduledBackup{
    ID:        "daily-backup",
    Name:      "daily",
    Schedule:  "@daily",
    Datastore: myDatastore,
    Enabled:   true,
}

scheduler.AddSchedule(schedule)
scheduler.Start()
```

## Configuration Options

### Backup Configuration

```go
type BackupConfig struct {
    CompressionLevel int           // Gzip compression (1-9)
    ChunkSize        int           // Records per processing chunk
    Timeout          time.Duration // Operation timeout
    VerifyIntegrity  bool          // Verify backup after creation
    IncludeMetadata  bool          // Include block metadata
    ExcludePatterns  []string      // Key patterns to exclude
}
```

### Migration Configuration

```go
type MigrationConfig struct {
    BatchSize       int           // Records per batch
    Timeout         time.Duration // Migration timeout
    VerifyMigration bool          // Verify after migration
    BackupBefore    bool          // Backup before migration
    DryRun          bool          // Simulate migration only
}
```

### Scheduler Configuration

```go
type SchedulerConfig struct {
    DefaultBackupDir    string           // Backup storage directory
    RetentionPolicy     RetentionPolicy  // Backup retention rules
    ConcurrentBackups   int              // Max concurrent operations
    HealthCheckInterval time.Duration    // Health check frequency
    NotificationConfig  NotificationConfig // Alert settings
}
```

## Advanced Features

### Data Transformation

Transform keys and values during migration:

```go
transform := backup.TransformationConfig{
    KeyTransform:   "add_prefix",
    ValueTransform: "compress",
    Mappings: map[string]string{
        "/old/path": "/new/path",
    },
    Validators: []string{"cid_validator", "size_validator"},
}
```

### Filtering

Exclude specific data patterns from backups:

```go
config := backup.DefaultBackupConfig()
config.ExcludePatterns = []string{
    "/temp/*",
    "/cache/*",
    "*.tmp",
}
```

### Retention Policies

Configure how long to keep backups:

```go
retention := backup.RetentionPolicy{
    KeepDaily:   7,   // 7 daily backups
    KeepWeekly:  4,   // 4 weekly backups
    KeepMonthly: 12,  // 12 monthly backups
    KeepYearly:  5,   // 5 yearly backups
    MaxAge:      365 * 24 * time.Hour, // 1 year max
}
```

## Monitoring and Metrics

All components provide comprehensive metrics:

```go
// Get backup metrics
backupMetrics := manager.GetMetrics()
fmt.Printf("Backup requests: %d\n", backupMetrics.TotalRequests)
fmt.Printf("Success rate: %.2f%%\n", backupMetrics.SuccessRate*100)

// Get migration metrics
migrationMetrics := migrationManager.GetMetrics()
fmt.Printf("Migration duration: %v\n", migrationMetrics.AverageDuration)

// Get scheduler metrics
schedulerMetrics := scheduler.GetMetrics()
fmt.Printf("Scheduled jobs: %d\n", len(scheduler.ListSchedules()))
```

## Error Handling

The package provides detailed error information:

```go
result, err := migrationManager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
if err != nil {
    log.Printf("Migration failed: %v\n", err)
}

if !result.Success {
    log.Printf("Migration completed with errors:\n")
    for _, errMsg := range result.ErrorLog {
        log.Printf("  - %s\n", errMsg)
    }
}
```

## Best Practices

### 1. Backup Strategy
- **Regular Schedules**: Use daily backups for active datasets
- **Compression**: Balance compression level vs speed
- **Verification**: Always verify critical backups
- **Retention**: Keep multiple backup generations

### 2. Migration Planning
- **Test First**: Always use dry-run mode for testing
- **Backup Before**: Create backups before major migrations
- **Validation**: Include validation steps in migration plans
- **Rollback**: Prepare rollback procedures

### 3. Monitoring
- **Health Checks**: Monitor backup and migration health
- **Alerts**: Set up notifications for failures
- **Metrics**: Track performance and success rates
- **Capacity**: Monitor storage space for backups

### 4. Security
- **Access Control**: Restrict access to backup files
- **Encryption**: Consider encrypting sensitive backups
- **Verification**: Regularly verify backup integrity
- **Retention**: Follow data retention compliance rules

## Integration Examples

### With Persistent Datastore

```go
import (
    "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
    "github.com/gosuda/boxo-starter-kit/pkg/backup"
)

// Create datastore
ds, err := persistent.NewBadgerDatastore("./data")
if err != nil {
    log.Fatal(err)
}

// Create backup
manager := backup.NewBackupManager(backup.DefaultBackupConfig())
metadata, err := manager.CreateBackup(ctx, ds, "datastore-backup.tar.gz")
```

### With Network Layer

```go
import (
    "github.com/gosuda/boxo-starter-kit/02-network/pkg"
    "github.com/gosuda/boxo-starter-kit/pkg/backup"
)

// Backup network configuration and peer data
networkWrapper, _ := network.NewHostWrapper(ctx, nil)
scheduler := backup.NewBackupScheduler(backup.DefaultSchedulerConfig())

// Schedule network state backups
schedule := &backup.ScheduledBackup{
    ID:        "network-backup",
    Schedule:  "@hourly",
    Datastore: networkWrapper.Host.Peerstore(),
    Enabled:   true,
}
scheduler.AddSchedule(schedule)
```

## Performance Considerations

- **Chunk Size**: Larger chunks = better performance, more memory usage
- **Compression**: Higher compression = smaller files, slower processing
- **Concurrency**: Balance concurrent operations with system resources
- **I/O**: Consider disk speed when setting batch sizes
- **Memory**: Monitor memory usage during large migrations

## Troubleshooting

### Common Issues

1. **Out of Memory**: Reduce chunk size or batch size
2. **Slow Backups**: Increase chunk size or reduce compression
3. **Migration Failures**: Check validation steps and filters
4. **Storage Full**: Implement retention policies
5. **Permission Errors**: Check file system permissions

### Debug Mode

Enable verbose logging for troubleshooting:

```go
config := backup.DefaultBackupConfig()
config.Timeout = 5 * time.Minute  // Increase timeout
// Add custom logging in your application
```

This backup and migration system provides enterprise-grade data management capabilities for IPFS datastores, ensuring data safety, migration flexibility, and operational automation.