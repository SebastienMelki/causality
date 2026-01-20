// Package warehouse provides data warehouse integration for event storage.
package warehouse

import (
	"time"
)

// Config holds warehouse sink configuration.
type Config struct {
	// S3 configuration
	S3 S3Config `envPrefix:"S3_"`

	// Batching configuration
	Batch BatchConfig `envPrefix:"BATCH_"`

	// Parquet configuration
	Parquet ParquetConfig `envPrefix:"PARQUET_"`
}

// S3Config holds S3/MinIO configuration.
type S3Config struct {
	// Endpoint is the S3 endpoint URL (e.g., "http://localhost:9000" for MinIO)
	Endpoint string `env:"ENDPOINT" envDefault:"http://localhost:9000"`

	// Region is the AWS region
	Region string `env:"REGION" envDefault:"us-east-1"`

	// Bucket is the S3 bucket name
	Bucket string `env:"BUCKET" envDefault:"causality-events"`

	// AccessKeyID is the AWS access key ID
	AccessKeyID string `env:"ACCESS_KEY_ID" envDefault:"minioadmin"`

	// SecretAccessKey is the AWS secret access key
	SecretAccessKey string `env:"SECRET_ACCESS_KEY" envDefault:"minioadmin"`

	// UsePathStyle enables path-style addressing (required for MinIO)
	UsePathStyle bool `env:"USE_PATH_STYLE" envDefault:"true"`

	// Prefix is the key prefix for all objects
	Prefix string `env:"PREFIX" envDefault:"events"`
}

// BatchConfig holds event batching configuration.
type BatchConfig struct {
	// MaxEvents is the maximum number of events per batch
	MaxEvents int `env:"MAX_EVENTS" envDefault:"10000"`

	// FlushInterval is the maximum time to wait before flushing a batch
	FlushInterval time.Duration `env:"FLUSH_INTERVAL" envDefault:"5m"`

	// MaxConcurrentWrites is the maximum number of concurrent S3 writes
	MaxConcurrentWrites int `env:"MAX_CONCURRENT_WRITES" envDefault:"4"`
}

// ParquetConfig holds Parquet writer configuration.
type ParquetConfig struct {
	// Compression is the compression codec (snappy, gzip, zstd, none)
	Compression string `env:"COMPRESSION" envDefault:"snappy"`

	// RowGroupSize is the number of rows per row group
	RowGroupSize int64 `env:"ROW_GROUP_SIZE" envDefault:"10000"`
}
