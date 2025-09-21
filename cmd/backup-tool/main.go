package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	badgerds "github.com/ipfs/go-ds-badger"
	pebbleds "github.com/ipfs/go-ds-pebble"

	"github.com/gosuda/boxo-starter-kit/pkg/backup"
)

// Command line tool for backup and migration operations
func main() {
	var (
		command        = flag.String("cmd", "", "Command: backup, restore, migrate, schedule, verify")
		datastorePath  = flag.String("datastore", "./data", "Path to datastore")
		datastoreType  = flag.String("type", "badger", "Datastore type: memory, file, badger, pebble")
		backupPath     = flag.String("backup", "", "Path to backup file")
		configPath     = flag.String("config", "", "Path to configuration file")
		compressionLevel = flag.Int("compression", 6, "Compression level (1-9)")
		chunkSize      = flag.Int("chunk-size", 1000, "Chunk size for processing")
		verify         = flag.Bool("verify", true, "Verify backup integrity")
		dryRun         = flag.Bool("dry-run", false, "Dry run mode (don't make changes)")
		schedule       = flag.String("schedule", "", "Cron schedule expression")
	)
	flag.Parse()

	if *command == "" {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()

	switch *command {
	case "backup":
		runBackup(ctx, *datastorePath, *datastoreType, *backupPath, *compressionLevel, *chunkSize, *verify)
	case "restore":
		runRestore(ctx, *backupPath, *datastorePath, *datastoreType)
	case "verify":
		runVerify(ctx, *backupPath)
	case "migrate":
		runMigrate(ctx, *configPath, *dryRun)
	case "schedule":
		runScheduler(ctx, *configPath, *schedule)
	case "info":
		runInfo(ctx, *backupPath)
	default:
		fmt.Printf("Unknown command: %s\n", *command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Backup and Migration Tool for IPFS Datastores")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  backup-tool -cmd=<command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  backup    Create a backup of a datastore")
	fmt.Println("  restore   Restore a datastore from backup")
	fmt.Println("  verify    Verify backup integrity")
	fmt.Println("  migrate   Execute a migration plan")
	fmt.Println("  schedule  Run backup scheduler")
	fmt.Println("  info      Show backup information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Create a backup")
	fmt.Println("  backup-tool -cmd=backup -datastore=./data -backup=backup.tar.gz")
	fmt.Println()
	fmt.Println("  # Restore from backup")
	fmt.Println("  backup-tool -cmd=restore -backup=backup.tar.gz -datastore=./restored")
	fmt.Println()
	fmt.Println("  # Verify backup")
	fmt.Println("  backup-tool -cmd=verify -backup=backup.tar.gz")
	fmt.Println()
	fmt.Println("  # Run migration")
	fmt.Println("  backup-tool -cmd=migrate -config=migration.json")
	fmt.Println()
	flag.PrintDefaults()
}

func runBackup(ctx context.Context, datastorePath, datastoreType, backupPath string, compressionLevel, chunkSize int, verify bool) {
	fmt.Printf("Creating backup of %s datastore at %s\n", datastoreType, datastorePath)

	// Open datastore
	ds, err := openDatastore(datastorePath, datastoreType)
	if err != nil {
		log.Fatalf("Failed to open datastore: %v", err)
	}
	defer ds.Close()

	// Create backup config
	config := backup.DefaultBackupConfig()
	config.CompressionLevel = compressionLevel
	config.ChunkSize = chunkSize
	config.VerifyIntegrity = verify

	// Create backup manager
	manager := backup.NewBackupManager(config)

	// Generate backup path if not provided
	if backupPath == "" {
		timestamp := time.Now().Format("20060102_150405")
		backupPath = fmt.Sprintf("backup_%s_%s.tar.gz", datastoreType, timestamp)
	}

	start := time.Now()
	fmt.Printf("Starting backup to %s...\n", backupPath)

	// Create backup
	metadata, err := manager.CreateBackup(ctx, ds, backupPath)
	if err != nil {
		log.Fatalf("Backup failed: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("Backup completed successfully!\n")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Total keys: %d\n", metadata.TotalKeys)
	fmt.Printf("Total size: %d bytes\n", metadata.TotalSize)
	fmt.Printf("Compressed size: %d bytes\n", metadata.Statistics.BytesCompressed)
	fmt.Printf("Compression ratio: %.2f%%\n", metadata.Statistics.CompressionRatio*100)

	if verify {
		fmt.Printf("Verifying backup...\n")
		_, err := manager.VerifyBackup(ctx, backupPath)
		if err != nil {
			log.Fatalf("Backup verification failed: %v", err)
		}
		fmt.Printf("Backup verification successful!\n")
	}
}

func runRestore(ctx context.Context, backupPath, datastorePath, datastoreType string) {
	fmt.Printf("Restoring backup from %s to %s datastore at %s\n", backupPath, datastoreType, datastorePath)

	// Create target datastore
	ds, err := createDatastore(datastorePath, datastoreType)
	if err != nil {
		log.Fatalf("Failed to create target datastore: %v", err)
	}
	defer ds.Close()

	// Create backup manager
	manager := backup.NewBackupManager(backup.DefaultBackupConfig())

	start := time.Now()
	fmt.Printf("Starting restore...\n")

	// Restore backup
	metadata, err := manager.RestoreBackup(ctx, backupPath, ds)
	if err != nil {
		log.Fatalf("Restore failed: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("Restore completed successfully!\n")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Restored keys: %d\n", metadata.TotalKeys)
	fmt.Printf("Original backup date: %v\n", metadata.Timestamp)
}

func runVerify(ctx context.Context, backupPath string) {
	fmt.Printf("Verifying backup: %s\n", backupPath)

	// Create backup manager
	manager := backup.NewBackupManager(backup.DefaultBackupConfig())

	start := time.Now()
	metadata, err := manager.VerifyBackup(ctx, backupPath)
	if err != nil {
		log.Fatalf("Verification failed: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("Verification completed successfully!\n")
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Backup version: %s\n", metadata.Version)
	fmt.Printf("Backup date: %v\n", metadata.Timestamp)
	fmt.Printf("Total keys: %d\n", metadata.TotalKeys)
	fmt.Printf("Total size: %d bytes\n", metadata.TotalSize)
}

func runMigrate(ctx context.Context, configPath string, dryRun bool) {
	if configPath == "" {
		log.Fatal("Migration config file required")
	}

	fmt.Printf("Running migration from config: %s\n", configPath)

	// Load migration plan
	plan, err := loadMigrationPlan(configPath)
	if err != nil {
		log.Fatalf("Failed to load migration plan: %v", err)
	}

	// Override dry run setting
	plan.Config.DryRun = dryRun

	if dryRun {
		fmt.Printf("DRY RUN MODE - No changes will be made\n")
	}

	fmt.Printf("Migration plan: %s (v%s)\n", plan.Description, plan.Version)
	fmt.Printf("Steps: %d\n", len(plan.Steps))

	// For this example, we'll create dummy datastores
	// In practice, these would be opened based on the plan configuration
	sourceDS, err := openDatastore("./source", "memory")
	if err != nil {
		log.Fatalf("Failed to open source datastore: %v", err)
	}
	defer sourceDS.Close()

	targetDS, err := createDatastore("./target", "memory")
	if err != nil {
		log.Fatalf("Failed to create target datastore: %v", err)
	}
	defer targetDS.Close()

	// Create migration manager
	manager := backup.NewMigrationManager(plan.Config)

	start := time.Now()
	fmt.Printf("Starting migration...\n")

	// Execute migration
	result, err := manager.ExecuteMigration(ctx, plan, sourceDS, targetDS)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	duration := time.Since(start)

	if result.Success {
		fmt.Printf("Migration completed successfully!\n")
	} else {
		fmt.Printf("Migration completed with errors!\n")
		for _, errMsg := range result.ErrorLog {
			fmt.Printf("  Error: %s\n", errMsg)
		}
	}

	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Total records: %d\n", result.Statistics.TotalRecords)
	fmt.Printf("Migrated records: %d\n", result.Statistics.MigratedRecords)
	fmt.Printf("Failed records: %d\n", result.Statistics.FailedRecords)
	fmt.Printf("Success rate: %.2f%%\n", result.Statistics.SuccessRate*100)
}

func runScheduler(ctx context.Context, configPath, scheduleExpr string) {
	fmt.Printf("Starting backup scheduler\n")

	// Create scheduler
	config := backup.DefaultSchedulerConfig()
	scheduler := backup.NewBackupScheduler(config)

	// Example: Add a simple scheduled backup
	if scheduleExpr != "" {
		ds, err := openDatastore("./data", "memory")
		if err != nil {
			log.Fatalf("Failed to open datastore: %v", err)
		}
		defer ds.Close()

		schedule := &backup.ScheduledBackup{
			ID:        "example-backup",
			Name:      "example",
			Schedule:  scheduleExpr,
			Datastore: ds,
			Enabled:   true,
		}

		err = scheduler.AddSchedule(schedule)
		if err != nil {
			log.Fatalf("Failed to add schedule: %v", err)
		}

		fmt.Printf("Added schedule: %s with expression %s\n", schedule.ID, scheduleExpr)
	}

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	fmt.Printf("Scheduler started. Press Ctrl+C to stop.\n")

	// Wait indefinitely (in practice, you'd handle signals)
	select {}
}

func runInfo(ctx context.Context, backupPath string) {
	fmt.Printf("Backup information for: %s\n", backupPath)

	// Create backup manager
	manager := backup.NewBackupManager(backup.DefaultBackupConfig())

	// Verify and get metadata
	metadata, err := manager.VerifyBackup(ctx, backupPath)
	if err != nil {
		log.Fatalf("Failed to read backup info: %v", err)
	}

	// Print detailed information
	fmt.Printf("\nBackup Metadata:\n")
	fmt.Printf("  Version: %s\n", metadata.Version)
	fmt.Printf("  Created: %v\n", metadata.Timestamp)
	fmt.Printf("  Total Keys: %d\n", metadata.TotalKeys)
	fmt.Printf("  Total Size: %d bytes\n", metadata.TotalSize)
	fmt.Printf("  Compression: %s\n", metadata.Compression)

	fmt.Printf("\nStatistics:\n")
	fmt.Printf("  Duration: %v\n", metadata.Statistics.Duration)
	fmt.Printf("  Keys Processed: %d\n", metadata.Statistics.KeysProcessed)
	fmt.Printf("  Bytes Processed: %d\n", metadata.Statistics.BytesProcessed)
	fmt.Printf("  Bytes Compressed: %d\n", metadata.Statistics.BytesCompressed)
	fmt.Printf("  Compression Ratio: %.2f%%\n", metadata.Statistics.CompressionRatio*100)
	fmt.Printf("  Error Count: %d\n", metadata.Statistics.ErrorCount)
	fmt.Printf("  Skipped Keys: %d\n", metadata.Statistics.SkippedKeys)

	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Compression Level: %d\n", metadata.Config.CompressionLevel)
	fmt.Printf("  Chunk Size: %d\n", metadata.Config.ChunkSize)
	fmt.Printf("  Verify Integrity: %t\n", metadata.Config.VerifyIntegrity)
	fmt.Printf("  Include Metadata: %t\n", metadata.Config.IncludeMetadata)

	if len(metadata.Config.ExcludePatterns) > 0 {
		fmt.Printf("  Exclude Patterns: %v\n", metadata.Config.ExcludePatterns)
	}
}

func openDatastore(path, dsType string) (datastore.Datastore, error) {
	switch dsType {
	case "memory":
		return dssync.MutexWrap(datastore.NewMapDatastore()), nil
	case "file":
		// Create directory if needed
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		return datastore.NewMapDatastore(), nil // Simple in-memory for demo
	case "badger":
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		return badgerds.NewDatastore(path, nil)
	case "pebble":
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		return pebbleds.NewDatastore(path)
	default:
		return nil, fmt.Errorf("unknown datastore type: %s", dsType)
	}
}

func createDatastore(path, dsType string) (datastore.Datastore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	return openDatastore(path, dsType)
}

func loadMigrationPlan(configPath string) (*backup.MigrationPlan, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var plan backup.MigrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}