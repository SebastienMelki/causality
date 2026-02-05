package dedup

import (
	"context"
	"log/slog"
	"time"

	"github.com/SebastienMelki/causality/internal/dedup/internal/service"
	"github.com/SebastienMelki/causality/internal/observability"
)

// Config holds the dedup module configuration.
//
// Environment variable overrides:
//   - DEDUP_WINDOW:   sliding window duration (default: 10m)
//   - DEDUP_CAPACITY: expected events per window (default: 1000000)
//   - DEDUP_FP_RATE:  bloom filter false positive rate (default: 0.0001)
type Config struct {
	Window   time.Duration `env:"DEDUP_WINDOW"   envDefault:"10m"`
	Capacity uint          `env:"DEDUP_CAPACITY" envDefault:"1000000"`
	FPRate   float64       `env:"DEDUP_FP_RATE"  envDefault:"0.0001"`
}

// DefaultConfig returns the default dedup configuration with a 10 minute
// sliding window, 1M event capacity, and 0.01% false positive rate.
func DefaultConfig() Config {
	return Config{
		Window:   10 * time.Minute,
		Capacity: 1_000_000,
		FPRate:   0.0001,
	}
}

// Module is the dedup module facade. It wraps the dedup service and
// provides a clean API for integration with the rest of the system.
type Module struct {
	svc *service.DedupService
}

// New creates a new dedup Module with the given configuration. The metrics
// parameter is optional (pass nil to disable metric instrumentation).
func New(cfg Config, metrics *observability.Metrics, logger *slog.Logger) *Module {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("module", "dedup")

	return &Module{
		svc: service.NewDedupService(cfg.Window, cfg.Capacity, cfg.FPRate, metrics, logger),
	}
}

// Start begins the background bloom filter rotation goroutine.
func (m *Module) Start(ctx context.Context) {
	m.svc.Start(ctx)
}

// Stop signals the rotation goroutine to stop and waits for completion.
func (m *Module) Stop() {
	m.svc.Stop()
}

// IsDuplicate checks whether the given idempotency key has been seen
// within the configured dedup window. Empty keys return false.
func (m *Module) IsDuplicate(key string) bool {
	return m.svc.IsDuplicate(key)
}
