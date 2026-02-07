package observability

import (
	otelmetric "go.opentelemetry.io/otel/metric"
)

// Metrics holds all metric instruments used across Causality services.
// Instruments are created once at startup and shared with middleware,
// handlers, and service components.
type Metrics struct {
	// HTTP metrics
	HTTPRequestDuration otelmetric.Float64Histogram
	HTTPRequestTotal    otelmetric.Int64Counter
	HTTPRequestErrors   otelmetric.Int64Counter

	// NATS metrics
	NATSMessagesProcessed otelmetric.Int64Counter
	NATSBatchSize         otelmetric.Int64Histogram
	NATSFlushLatency      otelmetric.Float64Histogram
	NATSAckLatency        otelmetric.Float64Histogram

	// S3 / storage metrics
	S3FilesWritten otelmetric.Int64Counter
	S3FileSize     otelmetric.Int64Histogram

	// Deduplication metrics
	DedupDropped otelmetric.Int64Counter

	// Dead-letter queue metrics
	DLQDepth otelmetric.Int64UpDownCounter

	// Compaction metrics
	CompactionRuns              otelmetric.Int64Counter
	CompactionFilesCompacted    otelmetric.Int64Counter
	CompactionPartitionsSkipped otelmetric.Int64Counter
	CompactionDuration          otelmetric.Float64Histogram

	// Reaction engine metrics
	RulesEvaluated otelmetric.Int64Counter
	AlertsFired    otelmetric.Int64Counter
	WebhookSuccess otelmetric.Int64Counter
	WebhookFailure otelmetric.Int64Counter
}

// NewMetrics creates all metric instruments from the given Meter.
// Each instrument is created with a descriptive name, unit, and description
// following OpenTelemetry semantic conventions.
func NewMetrics(meter otelmetric.Meter) (*Metrics, error) {
	var m Metrics
	var err error

	// HTTP metrics
	m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http.request.duration",
		otelmetric.WithUnit("ms"),
		otelmetric.WithDescription("HTTP request duration in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPRequestTotal, err = meter.Int64Counter(
		"http.request.total",
		otelmetric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPRequestErrors, err = meter.Int64Counter(
		"http.request.errors",
		otelmetric.WithDescription("HTTP request errors (4xx and 5xx)"),
	)
	if err != nil {
		return nil, err
	}

	// NATS metrics
	m.NATSMessagesProcessed, err = meter.Int64Counter(
		"nats.messages.processed",
		otelmetric.WithDescription("NATS messages processed"),
	)
	if err != nil {
		return nil, err
	}

	m.NATSBatchSize, err = meter.Int64Histogram(
		"nats.batch.size",
		otelmetric.WithDescription("NATS batch sizes"),
	)
	if err != nil {
		return nil, err
	}

	m.NATSFlushLatency, err = meter.Float64Histogram(
		"nats.flush.latency",
		otelmetric.WithUnit("ms"),
		otelmetric.WithDescription("Batch flush latency in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	m.NATSAckLatency, err = meter.Float64Histogram(
		"nats.ack.latency",
		otelmetric.WithUnit("ms"),
		otelmetric.WithDescription("NATS ACK latency in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	// S3 / storage metrics
	m.S3FilesWritten, err = meter.Int64Counter(
		"s3.files.written",
		otelmetric.WithDescription("S3 files written"),
	)
	if err != nil {
		return nil, err
	}

	m.S3FileSize, err = meter.Int64Histogram(
		"s3.file.size",
		otelmetric.WithUnit("By"),
		otelmetric.WithDescription("S3 file sizes in bytes"),
	)
	if err != nil {
		return nil, err
	}

	// Deduplication metrics
	m.DedupDropped, err = meter.Int64Counter(
		"dedup.dropped",
		otelmetric.WithDescription("Deduplicated events dropped"),
	)
	if err != nil {
		return nil, err
	}

	// Dead-letter queue metrics
	m.DLQDepth, err = meter.Int64UpDownCounter(
		"dlq.depth",
		otelmetric.WithDescription("Dead-letter queue message depth"),
	)
	if err != nil {
		return nil, err
	}

	// Compaction metrics
	m.CompactionRuns, err = meter.Int64Counter(
		"compaction.runs",
		otelmetric.WithDescription("Total compaction runs executed"),
	)
	if err != nil {
		return nil, err
	}

	m.CompactionFilesCompacted, err = meter.Int64Counter(
		"compaction.files.compacted",
		otelmetric.WithDescription("Total files merged during compaction"),
	)
	if err != nil {
		return nil, err
	}

	m.CompactionPartitionsSkipped, err = meter.Int64Counter(
		"compaction.partitions.skipped",
		otelmetric.WithDescription("Partitions skipped during compaction (no small files)"),
	)
	if err != nil {
		return nil, err
	}

	m.CompactionDuration, err = meter.Float64Histogram(
		"compaction.duration",
		otelmetric.WithUnit("ms"),
		otelmetric.WithDescription("Compaction run duration in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	// Reaction engine metrics
	m.RulesEvaluated, err = meter.Int64Counter(
		"rules.evaluated",
		otelmetric.WithDescription("Rules evaluated"),
	)
	if err != nil {
		return nil, err
	}

	m.AlertsFired, err = meter.Int64Counter(
		"alerts.fired",
		otelmetric.WithDescription("Alerts fired"),
	)
	if err != nil {
		return nil, err
	}

	m.WebhookSuccess, err = meter.Int64Counter(
		"webhook.success",
		otelmetric.WithDescription("Webhook deliveries successful"),
	)
	if err != nil {
		return nil, err
	}

	m.WebhookFailure, err = meter.Int64Counter(
		"webhook.failure",
		otelmetric.WithDescription("Webhook deliveries failed"),
	)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
