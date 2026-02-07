// Package observability provides OpenTelemetry-based metrics instrumentation
// with a Prometheus exporter for the Causality event processing system.
package observability

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Module holds the OTel MeterProvider and exposes a Meter for creating
// metric instruments. It is the central entry point for observability setup.
type Module struct {
	provider *sdkmetric.MeterProvider
	meter    otelmetric.Meter
}

// New creates a new observability Module. It configures a Prometheus exporter
// as the metric reader, creates a MeterProvider, and sets it as the global
// OTel MeterProvider. The serviceName is used as the meter scope name.
func New(serviceName string) (*Module, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(provider)

	meter := provider.Meter(serviceName)

	return &Module{
		provider: provider,
		meter:    meter,
	}, nil
}

// Shutdown gracefully shuts down the MeterProvider, flushing any remaining
// metric data.
func (m *Module) Shutdown(ctx context.Context) error {
	return m.provider.Shutdown(ctx)
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics
// in the standard exposition format. Mount this at "/metrics".
func (m *Module) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Meter returns the OTel Meter for creating metric instruments.
func (m *Module) Meter() otelmetric.Meter {
	return m.meter
}
