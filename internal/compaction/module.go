// Package compaction provides the Parquet file compaction module that merges
// small files into larger ones on a scheduled basis for improved Trino query
// performance.
//
// The warehouse sink produces many small Parquet files (one per batch flush).
// Over time, this degrades Trino query performance because it must open many
// small files. The compaction module periodically merges these small files into
// larger ones (target 128-256 MB), which significantly reduces the number of
// files Trino must scan.
//
// # Safety
//
//   - Only cold partitions (older than the current hour) are compacted.
//   - Original files are deleted ONLY after the compacted file is successfully uploaded.
//   - The service is stateless and idempotent: S3 file layout IS the state.
//   - If compaction fails partway, originals remain intact for the next run.
package compaction

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/SebastienMelki/causality/internal/compaction/internal/service"
	"github.com/SebastienMelki/causality/internal/observability"
	"github.com/SebastienMelki/causality/internal/warehouse"
)

// Config holds configuration for the compaction module.
type Config struct {
	// Enabled controls whether compaction is active.
	Enabled bool `env:"COMPACTION_ENABLED" envDefault:"true"`

	// Schedule is the interval between compaction runs.
	Schedule time.Duration `env:"COMPACTION_SCHEDULE" envDefault:"1h"`

	// TargetSize is the target file size for compacted files in bytes.
	// Default: 128 MB (134217728 bytes).
	TargetSize int64 `env:"COMPACTION_TARGET_SIZE" envDefault:"134217728"`

	// MinFiles is the minimum number of small files in a partition
	// required to trigger compaction.
	MinFiles int `env:"COMPACTION_MIN_FILES" envDefault:"2"`
}

// Module is the compaction module facade.
// It wraps the compaction service and scheduler, providing a clean public API
// with Start/Stop lifecycle and manual RunNow trigger.
type Module struct {
	svc       *service.CompactionService
	scheduler *service.Scheduler
	config    Config
	logger    *slog.Logger
}

// New creates a new compaction module.
//
// Parameters:
//   - s3Client: the raw AWS S3 client for listing, downloading, uploading, and deleting files
//   - s3Config: S3 configuration (bucket, prefix, etc.)
//   - cfg: compaction module configuration
//   - metrics: observability metrics (may be nil for no-op)
//   - logger: structured logger
func New(
	s3Client *s3.Client,
	s3Config warehouse.S3Config,
	cfg Config,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *Module {
	if logger == nil {
		logger = slog.Default()
	}

	compactionSvc := service.NewCompactionService(
		s3Client,
		s3Config,
		cfg.TargetSize,
		cfg.MinFiles,
		metrics,
		logger,
	)

	scheduler := service.NewScheduler(compactionSvc, cfg.Schedule, logger)

	return &Module{
		svc:       compactionSvc,
		scheduler: scheduler,
		config:    cfg,
		logger:    logger.With("component", "compaction-module"),
	}
}

// Start begins the scheduled compaction process.
// If compaction is disabled via config, this is a no-op.
func (m *Module) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("compaction disabled, skipping start")
		return nil
	}

	m.logger.Info("starting compaction module",
		"schedule", m.config.Schedule,
		"target_size", m.config.TargetSize,
		"min_files", m.config.MinFiles,
	)

	m.scheduler.Start(ctx)
	return nil
}

// Stop stops the compaction scheduler.
func (m *Module) Stop() {
	m.logger.Info("stopping compaction module")
	m.scheduler.Stop()
}

// RunNow triggers an immediate compaction run outside the scheduled interval.
func (m *Module) RunNow(ctx context.Context) error {
	return m.svc.CompactAll(ctx)
}
