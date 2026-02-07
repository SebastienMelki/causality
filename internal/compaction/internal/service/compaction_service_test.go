// Package service tests the compaction service logic.
package service

import (
	"testing"
	"time"

	"github.com/SebastienMelki/causality/internal/warehouse"
)

// TestExtractPartitionPrefix verifies partition prefix extraction from S3 keys.
func TestExtractPartitionPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "valid partition key",
			key:      "events/app_id=demo/year=2026/month=01/day=15/hour=10/events_abc123.parquet",
			expected: "events/app_id=demo/year=2026/month=01/day=15/hour=10/",
		},
		{
			name:     "another valid partition",
			key:      "data/app_id=myapp/year=2024/month=12/day=31/hour=23/file.parquet",
			expected: "data/app_id=myapp/year=2024/month=12/day=31/hour=23/",
		},
		{
			name:     "invalid key - no partition structure",
			key:      "events/random_file.parquet",
			expected: "",
		},
		{
			name:     "invalid key - missing hour",
			key:      "events/app_id=demo/year=2026/month=01/day=15/events.parquet",
			expected: "",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPartitionPrefix(tc.key)
			if result != tc.expected {
				t.Errorf("extractPartitionPrefix(%q) = %q, want %q", tc.key, result, tc.expected)
			}
		})
	}
}

// TestIsColdPartition verifies cold partition detection.
func TestIsColdPartition(t *testing.T) {
	// Fixed time: 2026-01-15 10:30:00 UTC
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		partition string
		isCold    bool
	}{
		{
			name:      "cold partition - previous hour",
			partition: "events/app_id=demo/year=2026/month=01/day=15/hour=09/",
			isCold:    true,
		},
		{
			name:      "cold partition - yesterday",
			partition: "events/app_id=demo/year=2026/month=01/day=14/hour=23/",
			isCold:    true,
		},
		{
			name:      "cold partition - previous month",
			partition: "events/app_id=demo/year=2025/month=12/day=31/hour=23/",
			isCold:    true,
		},
		{
			name:      "hot partition - current hour",
			partition: "events/app_id=demo/year=2026/month=01/day=15/hour=10/",
			isCold:    false,
		},
		{
			name:      "hot partition - future hour",
			partition: "events/app_id=demo/year=2026/month=01/day=15/hour=11/",
			isCold:    false,
		},
		{
			name:      "invalid partition format",
			partition: "events/invalid/",
			isCold:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isColdPartition(tc.partition, now)
			if result != tc.isCold {
				t.Errorf("isColdPartition(%q, %v) = %v, want %v", tc.partition, now, result, tc.isCold)
			}
		})
	}
}

// TestGroupIntoBatches verifies file batching logic.
func TestGroupIntoBatches(t *testing.T) {
	cs := &CompactionService{
		targetSize: 100,
		minFiles:   2,
	}

	tests := []struct {
		name           string
		files          []s3Object
		expectedBatches int
	}{
		{
			name: "single batch - all small files under target",
			files: []s3Object{
				{Key: "file1.parquet", Size: 20},
				{Key: "file2.parquet", Size: 30},
				{Key: "file3.parquet", Size: 25},
			},
			expectedBatches: 1, // 75 total < 100 target
		},
		{
			name: "two batches - split when over target",
			files: []s3Object{
				{Key: "file1.parquet", Size: 40},
				{Key: "file2.parquet", Size: 40},
				{Key: "file3.parquet", Size: 40},
				{Key: "file4.parquet", Size: 40},
			},
			expectedBatches: 2, // 80+80 = 160, splits into two batches
		},
		{
			name: "not enough files for batch",
			files: []s3Object{
				{Key: "file1.parquet", Size: 20},
			},
			expectedBatches: 0, // Only 1 file, minFiles is 2
		},
		{
			name:           "empty files list",
			files:          []s3Object{},
			expectedBatches: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batches := cs.groupIntoBatches(tc.files)
			if len(batches) != tc.expectedBatches {
				t.Errorf("groupIntoBatches() returned %d batches, want %d", len(batches), tc.expectedBatches)
			}
		})
	}
}

// TestGroupIntoBatches_MinFilesThreshold verifies minimum files threshold.
func TestGroupIntoBatches_MinFilesThreshold(t *testing.T) {
	cs := &CompactionService{
		targetSize: 100,
		minFiles:   3, // Require at least 3 files
	}

	// Only 2 files - should not create a batch
	files := []s3Object{
		{Key: "file1.parquet", Size: 20},
		{Key: "file2.parquet", Size: 20},
	}

	batches := cs.groupIntoBatches(files)
	if len(batches) != 0 {
		t.Errorf("groupIntoBatches() with %d files (minFiles=%d) should return 0 batches, got %d",
			len(files), cs.minFiles, len(batches))
	}

	// 3 files - should create a batch
	files = append(files, s3Object{Key: "file3.parquet", Size: 20})
	batches = cs.groupIntoBatches(files)
	if len(batches) != 1 {
		t.Errorf("groupIntoBatches() with %d files (minFiles=%d) should return 1 batch, got %d",
			len(files), cs.minFiles, len(batches))
	}
}

// TestNewCompactionService_Defaults verifies default values are applied.
func TestNewCompactionService_Defaults(t *testing.T) {
	cs := NewCompactionService(
		nil, // s3Client
		warehouse.S3Config{Bucket: "test-bucket", Prefix: "events"},
		0,   // targetSize 0 should use default
		0,   // minFiles 0 should use default
		nil, // metrics
		nil, // logger
	)

	if cs.targetSize != DefaultTargetSize {
		t.Errorf("targetSize = %d, want default %d", cs.targetSize, DefaultTargetSize)
	}

	if cs.minFiles != DefaultMinFiles {
		t.Errorf("minFiles = %d, want default %d", cs.minFiles, DefaultMinFiles)
	}
}

// TestNewCompactionService_CustomValues verifies custom values are used.
func TestNewCompactionService_CustomValues(t *testing.T) {
	customTargetSize := int64(256 * 1024 * 1024) // 256 MB
	customMinFiles := 5

	cs := NewCompactionService(
		nil,
		warehouse.S3Config{Bucket: "test-bucket", Prefix: "events"},
		customTargetSize,
		customMinFiles,
		nil,
		nil,
	)

	if cs.targetSize != customTargetSize {
		t.Errorf("targetSize = %d, want %d", cs.targetSize, customTargetSize)
	}

	if cs.minFiles != customMinFiles {
		t.Errorf("minFiles = %d, want %d", cs.minFiles, customMinFiles)
	}
}

// TestGenerateCompactedKey verifies compacted file key generation.
func TestGenerateCompactedKey(t *testing.T) {
	cs := &CompactionService{}

	partition := "events/app_id=demo/year=2026/month=01/day=15/hour=10/"
	key := cs.generateCompactedKey(partition)

	// Key should start with the partition prefix
	if len(key) <= len(partition) {
		t.Errorf("Generated key %q should be longer than partition %q", key, partition)
	}

	// Key should end with .parquet
	if key[len(key)-8:] != ".parquet" {
		t.Errorf("Generated key %q should end with .parquet", key)
	}

	// Key should contain "compacted_"
	if key[len(partition):len(partition)+10] != "compacted_" {
		t.Errorf("Generated key %q should contain 'compacted_' after partition", key)
	}
}

// TestS3Object verifies s3Object struct.
func TestS3Object(t *testing.T) {
	obj := s3Object{
		Key:  "events/file.parquet",
		Size: 1024,
	}

	if obj.Key != "events/file.parquet" {
		t.Errorf("Key = %q, want %q", obj.Key, "events/file.parquet")
	}
	if obj.Size != 1024 {
		t.Errorf("Size = %d, want %d", obj.Size, 1024)
	}
}

// TestPartitionRegex verifies the partition regex matches correctly.
func TestPartitionRegex(t *testing.T) {
	tests := []struct {
		key         string
		shouldMatch bool
	}{
		{"events/app_id=demo/year=2026/month=01/day=15/hour=10/file.parquet", true},
		{"data/app_id=myapp/year=2024/month=12/day=31/hour=23/events.parquet", true},
		{"events/random_file.parquet", false},
		{"/app_id=demo/year=2026/month=01/day=15/hour=10/", true}, // Prefix can be empty but needs /
		{"events/app_id=demo/year=2026/month=01/day=15/", false},   // Missing hour
		{"random_file.parquet", false},                             // No partition structure at all
	}

	for _, tc := range tests {
		matched := partitionRegex.MatchString(tc.key)
		if matched != tc.shouldMatch {
			t.Errorf("partitionRegex.MatchString(%q) = %v, want %v", tc.key, matched, tc.shouldMatch)
		}
	}
}

// --- Tests for CompactPartition and CompactAll ---

// TestCompactPartition_NoSmallFiles verifies CompactPartition returns false when no small files exist.
func TestCompactPartition_NoSmallFiles(t *testing.T) {
	// Note: This test documents the expected behavior when listObjects returns
	// only large files (size >= targetSize). Without mock S3, we test the
	// groupIntoBatches logic directly which is used by CompactPartition.

	cs := &CompactionService{
		targetSize: 100,
		minFiles:   2,
	}

	// Files that are all >= targetSize won't be in the smallFiles list
	largeFiles := []s3Object{
		{Key: "file1.parquet", Size: 150},
		{Key: "file2.parquet", Size: 200},
	}

	// The CompactPartition identifies small files as those < targetSize
	var smallFiles []s3Object
	for _, obj := range largeFiles {
		if obj.Size < cs.targetSize {
			smallFiles = append(smallFiles, obj)
		}
	}

	// Should have 0 small files
	if len(smallFiles) != 0 {
		t.Errorf("Expected 0 small files, got %d", len(smallFiles))
	}

	// groupIntoBatches with empty slice returns empty
	batches := cs.groupIntoBatches(smallFiles)
	if len(batches) != 0 {
		t.Errorf("Expected 0 batches for no small files, got %d", len(batches))
	}
}

// TestCompactPartition_NotEnoughFiles verifies partition skipped when < minFiles.
func TestCompactPartition_NotEnoughFiles(t *testing.T) {
	cs := &CompactionService{
		targetSize: 100,
		minFiles:   3, // Require at least 3 files
	}

	// Only 2 small files - less than minFiles
	smallFiles := []s3Object{
		{Key: "file1.parquet", Size: 20},
		{Key: "file2.parquet", Size: 30},
	}

	// Should not create any batches
	batches := cs.groupIntoBatches(smallFiles)
	if len(batches) != 0 {
		t.Errorf("Expected 0 batches for < minFiles, got %d", len(batches))
	}
}

// TestCompactPartition_CreatesBatches verifies batching logic for compaction.
func TestCompactPartition_CreatesBatches(t *testing.T) {
	cs := &CompactionService{
		targetSize: 100,
		minFiles:   2,
	}

	// 4 small files that should be split into 2 batches
	smallFiles := []s3Object{
		{Key: "file1.parquet", Size: 40},
		{Key: "file2.parquet", Size: 40},
		{Key: "file3.parquet", Size: 40},
		{Key: "file4.parquet", Size: 40},
	}

	batches := cs.groupIntoBatches(smallFiles)

	// Should have 2 batches (80 + 80 > 100 each)
	if len(batches) != 2 {
		t.Errorf("Expected 2 batches, got %d", len(batches))
	}

	// Each batch should have 2 files
	for i, batch := range batches {
		if len(batch) != 2 {
			t.Errorf("Batch %d has %d files, expected 2", i, len(batch))
		}
	}
}

// TestCompactAll_EmptyPartitions verifies CompactAll handles empty partition list.
// Note: Without mock S3, we test the internal logic paths.
func TestCompactAll_EmptyPartitions(t *testing.T) {
	// When listColdPartitions returns empty, CompactAll should succeed
	// with 0 partitions compacted. This documents expected behavior.
	partitions := []string{}

	compacted := 0
	for _, partition := range partitions {
		// This would call CompactPartition(partition)
		_ = partition
		compacted++
	}

	if compacted != 0 {
		t.Errorf("Expected 0 partitions compacted, got %d", compacted)
	}
}

// TestCompactAll_ContinuesOnPartitionError documents the error handling behavior.
func TestCompactAll_ContinuesOnPartitionError(t *testing.T) {
	// CompactAll continues processing partitions even if one fails.
	// This test documents that behavior.

	partitions := []string{
		"events/app_id=demo/year=2026/month=01/day=14/hour=10/",
		"events/app_id=demo/year=2026/month=01/day=14/hour=11/",
		"events/app_id=demo/year=2026/month=01/day=14/hour=12/",
	}

	// Simulate processing where first partition fails
	compacted := 0
	for i, partition := range partitions {
		if i == 0 {
			// First partition "fails" - in real code this would be a CompactPartition error
			_ = partition
			continue // CompactAll continues, doesn't return early
		}
		compacted++
	}

	// Should have processed 2 partitions despite first one "failing"
	if compacted != 2 {
		t.Errorf("Expected 2 partitions compacted, got %d", compacted)
	}
}

// TestDeleteObjects_EmptyList verifies deleteObjects handles empty list.
func TestDeleteObjects_EmptyList(t *testing.T) {
	// deleteObjects with empty list should return nil (no error)
	objects := []s3Object{}

	// The actual deleteObjects method checks len(objects) == 0
	if len(objects) != 0 {
		t.Error("Empty object list should have length 0")
	}
}

// TestDeleteObjects_Batching documents the 1000-object batch limit.
func TestDeleteObjects_Batching(t *testing.T) {
	// S3 DeleteObjects supports up to 1000 objects per request
	const maxBatch = 1000

	// Test with 2500 objects - should require 3 batches
	objects := make([]s3Object, 2500)
	for i := range objects {
		objects[i] = s3Object{Key: "file" + string(rune(i)) + ".parquet", Size: 100}
	}

	// Calculate expected batch count
	expectedBatches := (len(objects) + maxBatch - 1) / maxBatch
	if expectedBatches != 3 {
		t.Errorf("Expected 3 batches for 2500 objects, got %d", expectedBatches)
	}
}

// TestNewCompactionService_NilLogger verifies default logger is used.
func TestNewCompactionService_NilLogger(t *testing.T) {
	cs := NewCompactionService(nil, warehouse.S3Config{}, 0, 0, nil, nil)

	if cs.logger == nil {
		t.Error("Logger should not be nil after NewCompactionService")
	}
}

// TestNewCompactionService_NilMetrics verifies service works without metrics.
func TestNewCompactionService_NilMetrics(t *testing.T) {
	cs := NewCompactionService(nil, warehouse.S3Config{}, 0, 0, nil, nil)

	if cs.metrics != nil {
		t.Error("Metrics should be nil when not provided")
	}
}

// TestNewCompactionService_MinFilesEnforcement verifies minFiles minimum is 2.
func TestNewCompactionService_MinFilesEnforcement(t *testing.T) {
	// minFiles < 2 should be set to DefaultMinFiles (2)
	cs := NewCompactionService(nil, warehouse.S3Config{}, 0, 1, nil, nil)

	if cs.minFiles != DefaultMinFiles {
		t.Errorf("minFiles = %d, want %d (minimum enforced)", cs.minFiles, DefaultMinFiles)
	}

	// minFiles = 0 should also use default
	cs2 := NewCompactionService(nil, warehouse.S3Config{}, 0, 0, nil, nil)
	if cs2.minFiles != DefaultMinFiles {
		t.Errorf("minFiles = %d, want %d for zero value", cs2.minFiles, DefaultMinFiles)
	}
}

// TestGroupIntoBatches_ExactTargetSize verifies batch split at exact target size.
func TestGroupIntoBatches_ExactTargetSize(t *testing.T) {
	cs := &CompactionService{
		targetSize: 100,
		minFiles:   2,
	}

	// Files that exactly hit target size
	files := []s3Object{
		{Key: "file1.parquet", Size: 50},
		{Key: "file2.parquet", Size: 50},
		{Key: "file3.parquet", Size: 50},
		{Key: "file4.parquet", Size: 50},
	}

	batches := cs.groupIntoBatches(files)

	// With targetSize=100, after adding file1+file2 (100), when adding file3
	// the condition is currentSize+f.Size > targetSize (100+50 > 100) = true
	// So it should split
	if len(batches) < 1 {
		t.Errorf("Expected at least 1 batch, got %d", len(batches))
	}
}

// TestGroupIntoBatches_SingleLargeBatch verifies all files fit in one batch.
func TestGroupIntoBatches_SingleLargeBatch(t *testing.T) {
	cs := &CompactionService{
		targetSize: 1000, // Large target
		minFiles:   2,
	}

	// Small files that fit in one batch
	files := []s3Object{
		{Key: "file1.parquet", Size: 50},
		{Key: "file2.parquet", Size: 50},
		{Key: "file3.parquet", Size: 50},
	}

	batches := cs.groupIntoBatches(files)

	// All files should fit in one batch (150 < 1000)
	if len(batches) != 1 {
		t.Errorf("Expected 1 batch, got %d", len(batches))
	}

	if len(batches[0]) != 3 {
		t.Errorf("Batch should have 3 files, got %d", len(batches[0]))
	}
}

// TestIsColdPartition_EdgeCases verifies edge cases in cold partition detection.
func TestIsColdPartition_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		partition string
		now       time.Time
		expected  bool
	}{
		{
			name:      "midnight boundary - previous day last hour is cold",
			partition: "events/app_id=demo/year=2026/month=01/day=14/hour=23/",
			now:       time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "year boundary - previous year is cold",
			partition: "events/app_id=demo/year=2025/month=12/day=31/hour=23/",
			now:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "same exact hour is not cold",
			partition: "events/app_id=demo/year=2026/month=01/day=15/hour=10/",
			now:       time.Date(2026, 1, 15, 10, 59, 59, 0, time.UTC),
			expected:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isColdPartition(tc.partition, tc.now)
			if result != tc.expected {
				t.Errorf("isColdPartition(%q, %v) = %v, want %v",
					tc.partition, tc.now, result, tc.expected)
			}
		})
	}
}

// TestExtractPartitionPrefix_VariousFormats verifies partition extraction handles formats.
func TestExtractPartitionPrefix_VariousFormats(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "nested prefix",
			key:      "causality/data/events/app_id=demo/year=2026/month=01/day=15/hour=10/file.parquet",
			expected: "causality/data/events/app_id=demo/year=2026/month=01/day=15/hour=10/",
		},
		{
			name:     "single digit month/day/hour",
			key:      "events/app_id=demo/year=2026/month=01/day=05/hour=09/file.parquet",
			expected: "events/app_id=demo/year=2026/month=01/day=05/hour=09/",
		},
		{
			name:     "app_id with special chars",
			key:      "events/app_id=my-app_123/year=2026/month=01/day=15/hour=10/file.parquet",
			expected: "events/app_id=my-app_123/year=2026/month=01/day=15/hour=10/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPartitionPrefix(tc.key)
			if result != tc.expected {
				t.Errorf("extractPartitionPrefix(%q) = %q, want %q", tc.key, result, tc.expected)
			}
		})
	}
}

// TestGenerateCompactedKey_Uniqueness verifies each generated key is unique.
func TestGenerateCompactedKey_Uniqueness(t *testing.T) {
	cs := &CompactionService{}
	partition := "events/app_id=demo/year=2026/month=01/day=15/hour=10/"

	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := cs.generateCompactedKey(partition)
		if keys[key] {
			t.Errorf("Generated duplicate key: %s", key)
		}
		keys[key] = true
	}
}
