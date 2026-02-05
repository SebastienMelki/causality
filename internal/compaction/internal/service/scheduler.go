package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Scheduler runs compaction on a configurable interval using a ticker.
type Scheduler struct {
	svc      *CompactionService
	interval time.Duration
	logger   *slog.Logger

	mu      sync.Mutex
	stopCh  chan struct{}
	running bool
}

// NewScheduler creates a new compaction scheduler.
func NewScheduler(svc *CompactionService, interval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Hour
	}

	return &Scheduler{
		svc:      svc,
		interval: interval,
		logger:   logger.With("component", "compaction-scheduler"),
	}
}

// Start begins the scheduled compaction loop in a background goroutine.
// It runs the first compaction after one interval has elapsed, then
// repeats at the configured interval.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.logger.Warn("scheduler already running")
		return
	}

	s.stopCh = make(chan struct{})
	s.running = true

	go s.run(ctx)

	s.logger.Info("compaction scheduler started", "interval", s.interval)
}

// Stop signals the scheduler to stop and waits for the current run to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
	s.logger.Info("compaction scheduler stopped")
}

// RunNow triggers an immediate compaction run outside the scheduled interval.
func (s *Scheduler) RunNow(ctx context.Context) error {
	return s.svc.CompactAll(ctx)
}

// run is the main scheduler loop.
func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.logger.Info("scheduled compaction triggered")
			if err := s.svc.CompactAll(ctx); err != nil {
				s.logger.Error("scheduled compaction failed", "error", err)
			}
		}
	}
}
