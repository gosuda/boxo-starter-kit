package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// BackupManager handles backup and restore operations for IPFS datastores
type BackupManager struct {
	metrics *metrics.ComponentMetrics
	config  BackupConfig
}

// BackupConfig defines backup operation parameters
type BackupConfig struct {
	CompressionLevel int           // Gzip compression level (1-9)
	ChunkSize        int           // Number of records per chunk
	Timeout          time.Duration // Backup operation timeout
	VerifyIntegrity  bool          // Whether to verify backup integrity
	IncludeMetadata  bool          // Include block metadata in backup
	ExcludePatterns  []string      // Key patterns to exclude from backup
}

// DefaultBackupConfig returns sensible defaults
func DefaultBackupConfig() BackupConfig {
	return BackupConfig{
		CompressionLevel: 6,
		ChunkSize:        1000,
		Timeout:          30 * time.Minute,
		VerifyIntegrity:  true,
		IncludeMetadata:  true,
		ExcludePatterns:  []string{"/local/", "/temp/"},
	}
}

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	Version     string            `json:"version"`
	Timestamp   time.Time         `json:"timestamp"`
	TotalKeys   int64             `json:"total_keys"`
	TotalSize   int64             `json:"total_size"`
	Compression string            `json:"compression"`
	Checksum    string            `json:"checksum"`
	Config      BackupConfig      `json:"config"`
	Statistics  BackupStatistics  `json:"statistics"`
	DatastoreInfo map[string]interface{} `json:"datastore_info"`
}

// BackupStatistics tracks backup operation metrics
type BackupStatistics struct {
	Duration        time.Duration `json:"duration"`
	KeysProcessed   int64         `json:"keys_processed"`
	BytesProcessed  int64         `json:"bytes_processed"`
	BytesCompressed int64         `json:"bytes_compressed"`
	CompressionRatio float64       `json:"compression_ratio"`
	ErrorCount      int64         `json:"error_count"`
	SkippedKeys     int64         `json:"skipped_keys"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(config BackupConfig) *BackupManager {
	backupMetrics := metrics.NewComponentMetrics("backup_manager")
	metrics.RegisterGlobalComponent(backupMetrics)

	return &BackupManager{
		metrics: backupMetrics,
		config:  config,
	}
}

// CreateBackup creates a compressed backup of the datastore
func (bm *BackupManager) CreateBackup(ctx context.Context, ds datastore.Datastore, outputPath string) (*BackupMetadata, error) {
	start := time.Now()
	bm.metrics.RecordRequest()

	// Create backup context with timeout
	backupCtx, cancel := context.WithTimeout(ctx, bm.config.Timeout)
	defer cancel()

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "file_creation_failed")
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzipWriter, err := gzip.NewWriterLevel(file, bm.config.CompressionLevel)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "compression_init_failed")
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Initialize statistics
	stats := BackupStatistics{
		Duration: time.Since(start),
	}

	// Query all keys from datastore
	results, err := ds.Query(backupCtx, query.Query{})
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "datastore_query_failed")
		return nil, fmt.Errorf("failed to query datastore: %w", err)
	}
	defer results.Close()

	// Process entries in chunks
	chunk := make([]query.Result, 0, bm.config.ChunkSize)
	for result := range results.Next() {
		if result.Error != nil {
			stats.ErrorCount++
			continue
		}

		// Check if key should be excluded
		if bm.shouldExcludeKey(result.Entry.Key) {
			stats.SkippedKeys++
			continue
		}

		chunk = append(chunk, result)
		if len(chunk) >= bm.config.ChunkSize {
			if err := bm.writeChunk(tarWriter, chunk, &stats); err != nil {
				bm.metrics.RecordFailure(time.Since(start), "chunk_write_failed")
				return nil, fmt.Errorf("failed to write chunk: %w", err)
			}
			chunk = chunk[:0] // Reset slice
		}

		// Check for cancellation
		select {
		case <-backupCtx.Done():
			bm.metrics.RecordFailure(time.Since(start), "backup_cancelled")
			return nil, backupCtx.Err()
		default:
		}
	}

	// Write remaining entries
	if len(chunk) > 0 {
		if err := bm.writeChunk(tarWriter, chunk, &stats); err != nil {
			bm.metrics.RecordFailure(time.Since(start), "final_chunk_write_failed")
			return nil, fmt.Errorf("failed to write final chunk: %w", err)
		}
	}

	// Create metadata
	metadata := &BackupMetadata{
		Version:     "1.0",
		Timestamp:   start,
		TotalKeys:   stats.KeysProcessed,
		TotalSize:   stats.BytesProcessed,
		Compression: fmt.Sprintf("gzip-%d", bm.config.CompressionLevel),
		Config:      bm.config,
		Statistics:  stats,
		DatastoreInfo: map[string]interface{}{
			"type": fmt.Sprintf("%T", ds),
		},
	}

	// Calculate compression ratio
	if stats.BytesProcessed > 0 {
		stats.CompressionRatio = float64(stats.BytesCompressed) / float64(stats.BytesProcessed)
	}

	// Write metadata as JSON
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "metadata_marshal_failed")
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataHeader := &tar.Header{
		Name: "metadata.json",
		Mode: 0644,
		Size: int64(len(metadataBytes)),
	}

	if err := tarWriter.WriteHeader(metadataHeader); err != nil {
		bm.metrics.RecordFailure(time.Since(start), "metadata_header_write_failed")
		return nil, fmt.Errorf("failed to write metadata header: %w", err)
	}

	if _, err := tarWriter.Write(metadataBytes); err != nil {
		bm.metrics.RecordFailure(time.Since(start), "metadata_write_failed")
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	stats.Duration = time.Since(start)
	metadata.Statistics = stats

	bm.metrics.RecordSuccess(time.Since(start), stats.BytesProcessed)
	return metadata, nil
}

// RestoreBackup restores a datastore from a backup file
func (bm *BackupManager) RestoreBackup(ctx context.Context, backupPath string, ds datastore.Datastore) (*BackupMetadata, error) {
	start := time.Now()
	bm.metrics.RecordRequest()

	// Open backup file
	file, err := os.Open(backupPath)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "file_open_failed")
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "gzip_reader_failed")
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	var metadata *BackupMetadata
	restoredKeys := int64(0)

	// Process tar entries
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			bm.metrics.RecordFailure(time.Since(start), "tar_read_failed")
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Handle metadata
		if header.Name == "metadata.json" {
			metadataBytes, err := io.ReadAll(tarReader)
			if err != nil {
				bm.metrics.RecordFailure(time.Since(start), "metadata_read_failed")
				return nil, fmt.Errorf("failed to read metadata: %w", err)
			}

			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				bm.metrics.RecordFailure(time.Since(start), "metadata_unmarshal_failed")
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			continue
		}

		// Handle data chunks
		if filepath.Ext(header.Name) == ".chunk" {
			chunkData, err := io.ReadAll(tarReader)
			if err != nil {
				bm.metrics.RecordFailure(time.Since(start), "chunk_read_failed")
				return nil, fmt.Errorf("failed to read chunk: %w", err)
			}

			restored, err := bm.restoreChunk(ctx, ds, chunkData)
			if err != nil {
				bm.metrics.RecordFailure(time.Since(start), "chunk_restore_failed")
				return nil, fmt.Errorf("failed to restore chunk: %w", err)
			}
			restoredKeys += restored
		}

		// Check for cancellation
		select {
		case <-ctx.Done():
			bm.metrics.RecordFailure(time.Since(start), "restore_cancelled")
			return nil, ctx.Err()
		default:
		}
	}

	if metadata == nil {
		bm.metrics.RecordFailure(time.Since(start), "metadata_not_found")
		return nil, fmt.Errorf("backup metadata not found")
	}

	bm.metrics.RecordSuccess(time.Since(start), restoredKeys)
	return metadata, nil
}

// VerifyBackup verifies the integrity of a backup file
func (bm *BackupManager) VerifyBackup(ctx context.Context, backupPath string) (*BackupMetadata, error) {
	start := time.Now()
	bm.metrics.RecordRequest()

	// Open and parse backup
	file, err := os.Open(backupPath)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "file_open_failed")
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		bm.metrics.RecordFailure(time.Since(start), "gzip_reader_failed")
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	var metadata *BackupMetadata
	entriesFound := int64(0)
	bytesVerified := int64(0)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			bm.metrics.RecordFailure(time.Since(start), "tar_read_failed")
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		if header.Name == "metadata.json" {
			metadataBytes, err := io.ReadAll(tarReader)
			if err != nil {
				bm.metrics.RecordFailure(time.Since(start), "metadata_read_failed")
				return nil, fmt.Errorf("failed to read metadata: %w", err)
			}

			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				bm.metrics.RecordFailure(time.Since(start), "metadata_unmarshal_failed")
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			// Verify data chunks can be read
			_, err := io.ReadAll(tarReader)
			if err != nil {
				bm.metrics.RecordFailure(time.Since(start), "chunk_verification_failed")
				return nil, fmt.Errorf("failed to verify chunk %s: %w", header.Name, err)
			}
			entriesFound++
			bytesVerified += header.Size
		}
	}

	if metadata == nil {
		bm.metrics.RecordFailure(time.Since(start), "metadata_not_found")
		return nil, fmt.Errorf("backup metadata not found")
	}

	// Additional integrity checks could be added here
	// e.g., checksum verification, entry count validation

	bm.metrics.RecordSuccess(time.Since(start), bytesVerified)
	return metadata, nil
}

// shouldExcludeKey checks if a key should be excluded from backup
func (bm *BackupManager) shouldExcludeKey(key string) bool {
	for _, pattern := range bm.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, key); matched {
			return true
		}
	}
	return false
}

// writeChunk writes a chunk of datastore entries to the tar archive
func (bm *BackupManager) writeChunk(tarWriter *tar.Writer, chunk []query.Result, stats *BackupStatistics) error {
	chunkData := make(map[string][]byte)

	for _, result := range chunk {
		chunkData[result.Entry.Key] = result.Entry.Value
		stats.KeysProcessed++
		stats.BytesProcessed += int64(len(result.Entry.Value))
	}

	// Serialize chunk
	chunkBytes, err := json.Marshal(chunkData)
	if err != nil {
		return fmt.Errorf("failed to marshal chunk: %w", err)
	}

	// Create tar header
	chunkName := fmt.Sprintf("chunk_%d.chunk", stats.KeysProcessed/int64(bm.config.ChunkSize))
	header := &tar.Header{
		Name: chunkName,
		Mode: 0644,
		Size: int64(len(chunkBytes)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write chunk header: %w", err)
	}

	if _, err := tarWriter.Write(chunkBytes); err != nil {
		return fmt.Errorf("failed to write chunk data: %w", err)
	}

	stats.BytesCompressed += int64(len(chunkBytes))
	return nil
}

// restoreChunk restores a chunk of data to the datastore
func (bm *BackupManager) restoreChunk(ctx context.Context, ds datastore.Datastore, chunkData []byte) (int64, error) {
	var chunk map[string][]byte
	if err := json.Unmarshal(chunkData, &chunk); err != nil {
		return 0, fmt.Errorf("failed to unmarshal chunk: %w", err)
	}

	restoredCount := int64(0)
	for key, value := range chunk {
		dsKey := datastore.NewKey(key)
		if err := ds.Put(ctx, dsKey, value); err != nil {
			return restoredCount, fmt.Errorf("failed to put key %s: %w", key, err)
		}
		restoredCount++
	}

	return restoredCount, nil
}

// GetMetrics returns the current metrics for the backup manager
func (bm *BackupManager) GetMetrics() metrics.MetricsSnapshot {
	return bm.metrics.GetSnapshot()
}