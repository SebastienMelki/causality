// Package service provides the compaction service that merges small Parquet
// files into larger ones for improved query performance.
//
// The compaction service is stateless and idempotent. It uses the S3 file layout
// as its state: on each run it lists objects in cold partitions, identifies
// groups of small files, downloads them, merges their row groups into a single
// compacted file, uploads the result, and deletes the originals. If no small
// files are found, the run is a no-op.
package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/parquet-go/parquet-go"

	"github.com/SebastienMelki/causality/internal/observability"
	"github.com/SebastienMelki/causality/internal/warehouse"
)

// Default compaction parameters.
const (
	// DefaultTargetSize is the target compacted file size (128 MB).
	DefaultTargetSize int64 = 128 * 1024 * 1024

	// DefaultMinFiles is the minimum number of small files needed to trigger compaction.
	DefaultMinFiles int = 2
)

// s3Object represents a file in S3 with its key and size.
type s3Object struct {
	Key  string
	Size int64
}

// CompactionService merges small Parquet files into larger ones.
// It only operates on cold partitions (older than the current hour)
// and is safe to re-run (idempotent).
type CompactionService struct {
	s3Client   *s3.Client
	s3Config   warehouse.S3Config
	targetSize int64
	minFiles   int
	metrics    *observability.Metrics
	logger     *slog.Logger
}

// NewCompactionService creates a new compaction service.
func NewCompactionService(
	s3Client *s3.Client,
	s3Config warehouse.S3Config,
	targetSize int64,
	minFiles int,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *CompactionService {
	if logger == nil {
		logger = slog.Default()
	}
	if targetSize <= 0 {
		targetSize = DefaultTargetSize
	}
	if minFiles < 2 {
		minFiles = DefaultMinFiles
	}

	return &CompactionService{
		s3Client:   s3Client,
		s3Config:   s3Config,
		targetSize: targetSize,
		minFiles:   minFiles,
		metrics:    metrics,
		logger:     logger.With("component", "compaction-service"),
	}
}

// CompactAll lists all cold partitions and compacts each one.
// It records the CompactionRuns metric on each invocation.
func (cs *CompactionService) CompactAll(ctx context.Context) error {
	start := time.Now()
	cs.logger.Info("starting compaction run")

	partitions, err := cs.listColdPartitions(ctx)
	if err != nil {
		return fmt.Errorf("list cold partitions: %w", err)
	}

	cs.logger.Info("found cold partitions", "count", len(partitions))

	var compacted int
	for _, partition := range partitions {
		if err := ctx.Err(); err != nil {
			return err
		}

		did, compactErr := cs.CompactPartition(ctx, partition)
		if compactErr != nil {
			cs.logger.Error("failed to compact partition",
				"partition", partition,
				"error", compactErr,
			)
			// Continue with other partitions; don't fail the whole run.
			continue
		}
		if did {
			compacted++
		}
	}

	duration := float64(time.Since(start).Milliseconds())

	if cs.metrics != nil {
		cs.metrics.CompactionRuns.Add(ctx, 1)
		cs.metrics.CompactionDuration.Record(ctx, duration)
	}

	cs.logger.Info("compaction run complete",
		"partitions_total", len(partitions),
		"partitions_compacted", compacted,
		"duration_ms", duration,
	)

	return nil
}

// CompactPartition compacts a single partition by merging small files.
// Returns true if compaction was performed, false if skipped (no small files).
//
// Safety rules:
//   - NEVER compact the current hour partition (enforced by caller via listColdPartitions)
//   - Delete originals ONLY after successful upload
//   - On failure, leave originals intact
func (cs *CompactionService) CompactPartition(ctx context.Context, partition string) (bool, error) {
	// List all objects in the partition.
	objects, err := cs.listObjects(ctx, partition)
	if err != nil {
		return false, fmt.Errorf("list objects in partition %s: %w", partition, err)
	}

	// Identify small files (smaller than target size).
	var smallFiles []s3Object
	for _, obj := range objects {
		if obj.Size < cs.targetSize {
			smallFiles = append(smallFiles, obj)
		}
	}

	// Need at least minFiles small files to justify compaction.
	if len(smallFiles) < cs.minFiles {
		cs.logger.Debug("skipping partition, not enough small files",
			"partition", partition,
			"small_files", len(smallFiles),
			"min_required", cs.minFiles,
		)
		if cs.metrics != nil {
			cs.metrics.CompactionPartitionsSkipped.Add(ctx, 1)
		}
		return false, nil
	}

	cs.logger.Info("compacting partition",
		"partition", partition,
		"small_files", len(smallFiles),
	)

	// Group small files into batches that will produce files close to targetSize.
	batches := cs.groupIntoBatches(smallFiles)

	for batchIdx, batch := range batches {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		if err := cs.mergeBatch(ctx, partition, batch, batchIdx); err != nil {
			return false, fmt.Errorf("merge batch %d in partition %s: %w", batchIdx, partition, err)
		}
	}

	return true, nil
}

// groupIntoBatches groups small files into batches whose total size approaches targetSize.
func (cs *CompactionService) groupIntoBatches(files []s3Object) [][]s3Object {
	var batches [][]s3Object
	var currentBatch []s3Object
	var currentSize int64

	for _, f := range files {
		if currentSize+f.Size > cs.targetSize && len(currentBatch) >= cs.minFiles {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentSize = 0
		}
		currentBatch = append(currentBatch, f)
		currentSize += f.Size
	}

	// Add remaining files if we have enough for a batch.
	if len(currentBatch) >= cs.minFiles {
		batches = append(batches, currentBatch)
	}

	return batches
}

// mergeBatch downloads a batch of small Parquet files, merges their row groups,
// uploads the compacted file, and deletes the originals.
func (cs *CompactionService) mergeBatch(ctx context.Context, partition string, batch []s3Object, batchIdx int) error {
	cs.logger.Debug("merging batch",
		"partition", partition,
		"batch", batchIdx,
		"files", len(batch),
	)

	// Step 1: Download all small files and collect their row groups.
	var allRowGroups []parquet.RowGroup
	var downloadedFiles []*parquet.File

	for _, obj := range batch {
		data, err := cs.downloadObject(ctx, obj.Key)
		if err != nil {
			return fmt.Errorf("download %s: %w", obj.Key, err)
		}

		reader := bytes.NewReader(data)
		pf, err := parquet.OpenFile(reader, int64(len(data)))
		if err != nil {
			cs.logger.Warn("skipping corrupt parquet file",
				"key", obj.Key,
				"error", err,
			)
			continue
		}

		downloadedFiles = append(downloadedFiles, pf)
		allRowGroups = append(allRowGroups, pf.RowGroups()...)
	}

	if len(allRowGroups) == 0 {
		cs.logger.Warn("no row groups found in batch, skipping",
			"partition", partition,
			"batch", batchIdx,
		)
		return nil
	}

	// Step 2: Merge row groups.
	merged, err := parquet.MergeRowGroups(allRowGroups)
	if err != nil {
		return fmt.Errorf("merge row groups: %w", err)
	}

	// Step 3: Write merged data to a new Parquet file using the EventRow schema.
	var buf bytes.Buffer

	schema := parquet.SchemaOf(warehouse.EventRow{})
	writer := parquet.NewWriter(&buf,
		schema,
		parquet.Compression(&parquet.Snappy),
		parquet.CreatedBy("causality-compaction", "1.0.0", ""),
	)

	// Copy merged rows into the writer.
	rowReader := parquet.NewRowGroupReader(merged)
	rowBuf := make([]parquet.Row, 1000)
	for {
		n, readErr := rowReader.ReadRows(rowBuf)
		if n > 0 {
			if _, writeErr := writer.WriteRows(rowBuf[:n]); writeErr != nil {
				return fmt.Errorf("write merged rows: %w", writeErr)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("read merged rows: %w", readErr)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close compacted writer: %w", err)
	}

	// Step 4: Upload the compacted file.
	compactedKey := cs.generateCompactedKey(partition)
	compactedData := buf.Bytes()

	if _, err := cs.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cs.s3Config.Bucket),
		Key:         aws.String(compactedKey),
		Body:        bytes.NewReader(compactedData),
		ContentType: aws.String("application/x-parquet"),
	}); err != nil {
		return fmt.Errorf("upload compacted file %s: %w", compactedKey, err)
	}

	cs.logger.Info("uploaded compacted file",
		"key", compactedKey,
		"size_bytes", len(compactedData),
		"source_files", len(batch),
	)

	// Step 5: Delete originals ONLY after successful upload.
	if err := cs.deleteObjects(ctx, batch); err != nil {
		// Log but don't fail: the compacted file exists, so next run
		// may create a duplicate, but data is not lost. The duplicate
		// will be cleaned up on subsequent compaction runs because the
		// originals will eventually be deleted or the compacted file
		// won't meet smallFiles threshold.
		cs.logger.Error("failed to delete original files after compaction",
			"partition", partition,
			"error", err,
		)
	}

	if cs.metrics != nil {
		cs.metrics.CompactionFilesCompacted.Add(ctx, int64(len(batch)))
	}

	return nil
}

// listColdPartitions returns S3 prefixes for partitions that are older than
// the current hour. It walks the Hive-style partition tree:
// {prefix}/app_id=X/year=Y/month=M/day=D/hour=H/
func (cs *CompactionService) listColdPartitions(ctx context.Context) ([]string, error) {
	now := time.Now().UTC()

	// List all objects with the configured prefix and find unique partition prefixes.
	paginator := s3.NewListObjectsV2Paginator(cs.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(cs.s3Config.Bucket),
		Prefix: aws.String(cs.s3Config.Prefix + "/"),
	})

	partitionSet := make(map[string]struct{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			partition := extractPartitionPrefix(*obj.Key)
			if partition == "" {
				continue
			}

			// Check if this partition is cold (older than current hour).
			if isColdPartition(partition, now) {
				partitionSet[partition] = struct{}{}
			}
		}
	}

	partitions := make([]string, 0, len(partitionSet))
	for p := range partitionSet {
		partitions = append(partitions, p)
	}

	return partitions, nil
}

// partitionRegex matches Hive-style partition paths and extracts date components.
var partitionRegex = regexp.MustCompile(
	`(.*?/app_id=[^/]+/year=(\d{4})/month=(\d{2})/day=(\d{2})/hour=(\d{2})/)`,
)

// extractPartitionPrefix extracts the partition prefix from an S3 key.
// For example, "events/app_id=demo/year=2026/month=01/day=15/hour=10/events_uuid.parquet"
// returns "events/app_id=demo/year=2026/month=01/day=15/hour=10/".
func extractPartitionPrefix(key string) string {
	matches := partitionRegex.FindStringSubmatch(key)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// isColdPartition checks whether a partition is older than the current hour.
func isColdPartition(partition string, now time.Time) bool {
	matches := partitionRegex.FindStringSubmatch(partition)
	if len(matches) < 6 {
		return false
	}

	year, _ := strconv.Atoi(matches[2])
	month, _ := strconv.Atoi(matches[3])
	day, _ := strconv.Atoi(matches[4])
	hour, _ := strconv.Atoi(matches[5])

	partitionTime := time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC)
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)

	return partitionTime.Before(currentHour)
}

// listObjects lists all objects within a given partition prefix.
func (cs *CompactionService) listObjects(ctx context.Context, partition string) ([]s3Object, error) {
	paginator := s3.NewListObjectsV2Paginator(cs.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(cs.s3Config.Bucket),
		Prefix: aws.String(partition),
	})

	var objects []s3Object
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects in %s: %w", partition, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".parquet") {
				objects = append(objects, s3Object{
					Key:  *obj.Key,
					Size: *obj.Size,
				})
			}
		}
	}

	return objects, nil
}

// downloadObject downloads an S3 object and returns its contents.
func (cs *CompactionService) downloadObject(ctx context.Context, key string) ([]byte, error) {
	result, err := cs.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cs.s3Config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read object %s: %w", key, err)
	}

	return data, nil
}

// deleteObjects deletes the specified S3 objects.
func (cs *CompactionService) deleteObjects(ctx context.Context, objects []s3Object) error {
	if len(objects) == 0 {
		return nil
	}

	// S3 DeleteObjects supports up to 1000 objects per request.
	const maxBatch = 1000
	for i := 0; i < len(objects); i += maxBatch {
		end := i + maxBatch
		if end > len(objects) {
			end = len(objects)
		}

		identifiers := make([]s3types.ObjectIdentifier, 0, end-i)
		for _, obj := range objects[i:end] {
			identifiers = append(identifiers, s3types.ObjectIdentifier{
				Key: aws.String(obj.Key),
			})
		}

		_, err := cs.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(cs.s3Config.Bucket),
			Delete: &s3types.Delete{
				Objects: identifiers,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("delete objects: %w", err)
		}
	}

	return nil
}

// generateCompactedKey generates an S3 key for a compacted file in the given partition.
func (cs *CompactionService) generateCompactedKey(partition string) string {
	return fmt.Sprintf("%scompacted_%s.parquet", partition, uuid.New().String())
}
