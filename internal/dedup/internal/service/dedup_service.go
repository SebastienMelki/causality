// Package service implements the deduplication service that wraps the
// bloom filter domain with lifecycle management and metrics.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/SebastienMelki/causality/internal/dedup/internal/domain"
	"github.com/SebastienMelki/causality/internal/observability"
)

// DedupService manages the bloom filter lifecycle including periodic
// rotation and exposes the IsDuplicate check with metrics instrumentation.
type DedupService struct {
	filter  *domain.BloomFilterSet
	metrics *observability.Metrics
	logger  *slog.Logger
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewDedupService creates a new dedup service with the given bloom filter
// parameters. The metrics parameter is optional (can be nil) and logger
// is used for rotation lifecycle logging.
func NewDedupService(
	window time.Duration,
	capacity uint,
	fpRate float64,
	metrics *observability.Metrics,
	logger *slog.Logger,
) *DedupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DedupService{
		filter:  domain.NewBloomFilterSet(window, capacity, fpRate),
		metrics: metrics,
		logger:  logger,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// IsDuplicate checks whether the given idempotency key has been seen
// within the dedup window. Empty keys always return false (events without
// idempotency keys pass through unchanged). Duplicate detections are
// counted via the DedupDropped metric when metrics are configured.
func (s *DedupService) IsDuplicate(key string) bool {
	if key == "" {
		return false
	}

	if s.filter.IsDuplicate(key) {
		if s.metrics != nil {
			s.metrics.DedupDropped.Add(context.Background(), 1)
		}
		s.logger.Debug("duplicate event dropped", "idempotency_key", key)
		return true
	}

	return false
}

// Start launches the background goroutine that rotates the bloom filter
// every window/2 to maintain the sliding window. The goroutine stops when
// ctx is cancelled or Stop is called.
func (s *DedupService) Start(ctx context.Context) {
	rotateInterval := s.filter.Window() / 2
	s.logger.Info("dedup service started",
		"window", s.filter.Window(),
		"rotate_interval", rotateInterval,
	)

	go func() {
		defer close(s.doneCh)
		ticker := time.NewTicker(rotateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.filter.Rotate()
				s.logger.Debug("bloom filter rotated")
			case <-ctx.Done():
				s.logger.Info("dedup service stopping (context cancelled)")
				return
			case <-s.stopCh:
				s.logger.Info("dedup service stopping (stop requested)")
				return
			}
		}
	}()
}

// Stop signals the rotation goroutine to stop and waits for it to finish.
func (s *DedupService) Stop() {
	close(s.stopCh)
	<-s.doneCh
}
