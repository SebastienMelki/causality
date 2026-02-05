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
