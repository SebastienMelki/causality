# Phase 1: Pipeline Hardening, Observability & Go SDK - Research

**Researched:** 2026-02-05
**Domain:** Stream processing reliability, observability, API security, Go SDK design
**Confidence:** HIGH

## Summary

Phase 1 hardens the existing Go event pipeline (HTTP server, warehouse-sink, reaction-engine) by fixing data loss risks, adding authentication, observability, deduplication, dead letter queues, rate limiting, Parquet compaction, and shipping a Go backend SDK. The existing codebase already uses `log/slog`, NATS JetStream (`github.com/nats-io/nats.go v1.39.1`), `parquet-go/parquet-go v0.25.0`, AWS SDK v2 for S3, and `golang.org/x/time/rate` for rate limiting.

All 10 requirements (R1.1-R1.10) are achievable with the Go standard library plus well-established open-source packages. No novel technology is needed. The main technical challenge is the ACK-after-write refactor (R1.1/R1.6) which requires restructuring the warehouse consumer to track per-message NATS acknowledgments through the batch-flush-upload cycle.

**Primary recommendation:** Refactor the warehouse consumer first (R1.1 + R1.6), since ACK-after-write is the foundation for reliable processing. All other requirements can build on top of a correctly-flowing pipeline.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.opentelemetry.io/otel` | v1.40.0 | Metrics API | Official CNCF observability standard for Go |
| `go.opentelemetry.io/otel/exporters/prometheus` | v0.62.0 | Prometheus exporter | Standard bridge from OTel to Prometheus scraping |
| `go.opentelemetry.io/otel/sdk/metric` | latest | MeterProvider | Required to create meters and register exporter |
| `github.com/prometheus/client_golang` | latest | `/metrics` handler | Serves Prometheus exposition format via HTTP |
| `github.com/bits-and-blooms/bloom/v3` | v3.7.1 | Bloom filter for dedup | 197 importers, used by Milvus/Beego, well-maintained |
| `github.com/golang-migrate/migrate/v4` | v4.18.2+ | SQL migrations | Standard Go migration tool, plain SQL files |
| `golang.org/x/time/rate` | (already in go.mod) | Rate limiting | Standard library extension, token bucket algorithm |
| `github.com/google/uuid` | (already in go.mod) | Idempotency keys | Already used for event IDs |
| `golang.org/x/crypto/bcrypt` | latest | API key hashing | Standard bcrypt implementation via Go crypto |
| `github.com/parquet-go/parquet-go` | v0.25.0 -> v0.27.0 | Parquet read/write/merge | Already in use; upgrade for MergeRowGroups improvements |
| `log/slog` | stdlib | Structured logging | Already in use throughout codebase |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.opentelemetry.io/contrib/bridges/otelslog` | latest | slog-to-OTel bridge | Inject trace_id/span_id into slog output |
| `crypto/sha256` | stdlib | API key hashing alternative | If bcrypt is too slow for per-request auth (SHA256 + constant-time compare) |
| `crypto/subtle` | stdlib | Timing-safe comparison | Compare hashed API keys without timing leaks |
| `sync` | stdlib | Mutex, WaitGroup, Map | Per-key rate limiter storage, worker pool coordination |
| `database/sql` + `github.com/lib/pq` | (already in go.mod) | PostgreSQL | API key storage, migration state |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `bits-and-blooms/bloom` | `tylertreat/BoomFilters` (Stable Bloom) | BoomFilters handles unbounded streams with automatic decay; good for longer-running dedup windows. bloom/v3 is simpler and sufficient for 10-min sliding window with periodic rotation. |
| bcrypt for API keys | SHA256 + HMAC | bcrypt is slow by design (~100ms per verify). For per-request API key auth, SHA256 hash comparison is faster (~microseconds) and sufficient since API keys are long random strings, not user passwords. **Use SHA256 for API key storage, not bcrypt.** |
| golang-migrate | goose, atlas | golang-migrate is the most widely used, works with plain SQL files, and supports both CLI and library usage. |
| parquet-go MergeRowGroups | Apache Spark / external compactor | Go-native compaction avoids external JVM dependencies. parquet-go MergeRowGroups is designed for this exact use case. |

**Installation:**
```bash
# New dependencies to add
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk/metric@latest
go get go.opentelemetry.io/otel/exporters/prometheus@latest
go get github.com/prometheus/client_golang/prometheus/promhttp@latest
go get github.com/bits-and-blooms/bloom/v3@latest
go get github.com/golang-migrate/migrate/v4@latest
go get golang.org/x/crypto@latest

# Upgrade existing dependency
go get github.com/parquet-go/parquet-go@v0.27.0
```

## Architecture Patterns

### Recommended Module Structure (Hexagonal Pattern)

New modules for Phase 1 follow the established retcons pattern:

```
internal/
  auth/                        # NEW MODULE: API key authentication
    module.go                  # Facade: Module struct, New(), wiring
    ports.go                   # KeyStore interface, KeyValidator interface
    adapters.go                # Middleware adapter for gateway
    internal/
      domain/                  # APIKey value object, hashing logic
      service/                 # Key validation, creation, revocation
      repo/                    # PostgreSQL key storage
    migrations/                # 001_create_api_keys.up.sql, etc.

  dedup/                       # NEW MODULE: Event deduplication
    module.go                  # Facade
    ports.go                   # Deduplicator interface
    adapters.go                # Gateway middleware adapter
    internal/
      domain/                  # BloomFilterSet, sliding window logic
      service/                 # Dedup service with rotation

  compaction/                  # NEW MODULE: Parquet compaction
    module.go                  # Facade
    internal/
      service/                 # Compaction scheduler, merge logic
      repo/                    # S3 file listing, Hive metastore client

  observability/               # NEW MODULE: Metrics + logging setup
    module.go                  # MeterProvider, metric registration
    metrics.go                 # Metric definitions (counters, histograms)
    middleware.go              # HTTP metrics middleware

  dlq/                         # NEW MODULE: Dead letter queue
    module.go                  # Facade
    internal/
      service/                 # DLQ consumer, alerting

  gateway/                     # EXISTING: Modified for auth, rate limiting, validation
    middleware.go              # Add: AuthMiddleware, PerKeyRateLimit, BodySizeLimit
    server.go                  # Add: /metrics endpoint
    service.go                 # Add: validation, idempotency key injection

  warehouse/                   # EXISTING: Refactored for ACK-after-write
    consumer.go                # Refactor: worker pool, ACK tracking, DLQ
    compaction.go              # NEW: compaction job runner

  nats/                        # EXISTING: Modified for DLQ stream
    stream.go                  # Add: DLQ stream creation

  reaction/                    # EXISTING: Add metrics, fix silent errors
    consumer.go                # Add: metrics, proper error handling

sdk/
  go/                          # NEW: Go backend SDK
    causality.go               # Client struct, Track, Flush, Close
    transport.go               # HTTP transport with retry
    batch.go                   # Batching logic
    config.go                  # Configuration
```

### Pattern 1: ACK-After-Write (R1.1 + R1.6)

**What:** Track individual NATS messages through the batch-flush-upload cycle so they are only ACKed after successful S3 write.
**When to use:** Any consumer that must guarantee at-least-once processing.

```go
// Source: NATS JetStream docs + warehouse consumer refactor
// Key insight: messages must be tracked alongside batch events

type trackedEvent struct {
    event *pb.EventEnvelope
    msg   jetstream.Msg // retain NATS message reference for ACK/NAK
}

type Consumer struct {
    // ...
    batch []trackedEvent // track messages with events
}

func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg) error {
    var event pb.EventEnvelope
    if err := proto.Unmarshal(msg.Data(), &event); err != nil {
        // Unmarshal failure = poison message -> send to DLQ, then Term()
        msg.Term()
        return fmt.Errorf("unmarshal failed: %w", err)
    }

    c.mu.Lock()
    c.batch = append(c.batch, trackedEvent{event: &event, msg: msg})
    shouldFlush := len(c.batch) >= c.config.Batch.MaxEvents
    c.mu.Unlock()

    if shouldFlush {
        return c.flush(ctx)
    }
    return nil
}

func (c *Consumer) flush(ctx context.Context) error {
    c.mu.Lock()
    if len(c.batch) == 0 {
        c.mu.Unlock()
        return nil
    }
    tracked := c.batch
    c.batch = make([]trackedEvent, 0, c.config.Batch.MaxEvents)
    c.mu.Unlock()

    // Write to S3 (partitioned)
    if err := c.writePartitions(ctx, tracked); err != nil {
        // NAK all messages so they can be redelivered
        for _, t := range tracked {
            _ = t.msg.Nak()
        }
        return err
    }

    // S3 write succeeded -> ACK all messages
    for _, t := range tracked {
        if err := t.msg.Ack(); err != nil {
            c.logger.Error("failed to ACK after successful write", "error", err)
        }
    }
    return nil
}
```

**Critical detail:** In the current code, messages are ACKed immediately after `processMessage` returns successfully (line 116 in consumer.go), but the event is only added to the batch -- not yet written to S3. This means a crash between ACK and flush loses data. The refactor moves ACK to after flush.

### Pattern 2: Pull Consumer Worker Pool (R1.6)

**What:** Multiple goroutines fetch from the same durable pull consumer for parallel processing.
**When to use:** When single-goroutine consumption is a throughput bottleneck.

```go
// Source: NATS docs - pull consumers scale horizontally automatically
// Multiple Consume() calls on same consumer distribute messages

func (c *Consumer) Start(ctx context.Context) error {
    stream, err := c.js.Stream(ctx, c.streamName)
    if err != nil {
        return err
    }
    consumer, err := stream.Consumer(ctx, c.consumerName)
    if err != nil {
        return err
    }

    var wg sync.WaitGroup
    for i := 0; i < c.config.WorkerCount; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            c.workerLoop(ctx, consumer, workerID)
        }(i)
    }

    go func() {
        wg.Wait()
        close(c.doneCh)
    }()

    return nil
}

func (c *Consumer) workerLoop(ctx context.Context, consumer jetstream.Consumer, id int) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-c.stopCh:
            return
        default:
            msgs, err := consumer.Fetch(c.config.FetchBatchSize,
                jetstream.FetchMaxWait(5*time.Second))
            if err != nil {
                if !errors.Is(err, context.DeadlineExceeded) {
                    c.logger.Error("fetch error", "worker", id, "error", err)
                    time.Sleep(time.Second) // backoff on error
                }
                continue
            }
            for msg := range msgs.Messages() {
                c.processMessage(ctx, msg)
            }
        }
    }
}
```

### Pattern 3: Sliding Window Bloom Filter Deduplication (R1.2)

**What:** Two bloom filters rotated on a timer to create a sliding dedup window.
**When to use:** Server-side dedup of events with idempotency keys.

```go
// Source: bits-and-blooms/bloom v3.7.1
// Sliding window: maintain current + previous bloom filter
// Rotate every window/2 to create overlap

type Deduplicator struct {
    mu       sync.RWMutex
    current  *bloom.BloomFilter
    previous *bloom.BloomFilter
    window   time.Duration
    capacity uint       // expected items per window
    fpRate   float64    // false positive rate
}

func NewDeduplicator(window time.Duration, capacity uint, fpRate float64) *Deduplicator {
    return &Deduplicator{
        current:  bloom.NewWithEstimates(capacity, fpRate),
        previous: bloom.NewWithEstimates(capacity, fpRate),
        window:   window,
        capacity: capacity,
        fpRate:   fpRate,
    }
}

// IsDuplicate returns true if the key was seen in the current window.
// Thread-safe for concurrent use.
func (d *Deduplicator) IsDuplicate(key string) bool {
    keyBytes := []byte(key)
    d.mu.RLock()
    inCurrent := d.current.Test(keyBytes)
    inPrevious := d.previous.Test(keyBytes)
    d.mu.RUnlock()

    if inCurrent || inPrevious {
        return true
    }

    d.mu.Lock()
    d.current.Add(keyBytes)
    d.mu.Unlock()
    return false
}

// Rotate swaps current->previous and creates fresh current.
// Call every window/2 from a background goroutine.
func (d *Deduplicator) Rotate() {
    d.mu.Lock()
    d.previous = d.current
    d.current = bloom.NewWithEstimates(d.capacity, d.fpRate)
    d.mu.Unlock()
}
```

### Pattern 4: Per-API-Key Rate Limiting (R1.8)

**What:** Each API key gets its own token bucket rate limiter, stored in a sync.Map.
**When to use:** Per-tenant rate limiting without external infrastructure.

```go
// Source: golang.org/x/time/rate (already in go.mod)
// Per-key limiter with lazy initialization and periodic cleanup

type PerKeyRateLimiter struct {
    limiters sync.Map // map[string]*rate.Limiter
    rps      float64
    burst    int
}

func NewPerKeyRateLimiter(rps float64, burst int) *PerKeyRateLimiter {
    return &PerKeyRateLimiter{rps: rps, burst: burst}
}

func (l *PerKeyRateLimiter) Allow(key string) bool {
    v, _ := l.limiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(l.rps), l.burst))
    limiter := v.(*rate.Limiter)
    return limiter.Allow()
}
```

### Pattern 5: OpenTelemetry Prometheus Metrics (R1.5)

**What:** Register OTel MeterProvider with Prometheus exporter, expose `/metrics`.
**When to use:** All services.

```go
// Source: go.opentelemetry.io/otel/exporters/prometheus v0.62.0

import (
    "net/http"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/prometheus"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupMetrics() (*sdkmetric.MeterProvider, error) {
    exporter, err := prometheus.New()
    if err != nil {
        return nil, err
    }
    provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
    otel.SetMeterProvider(provider)
    return provider, nil
}

// Add to HTTP mux:
// mux.Handle("GET /metrics", promhttp.Handler())
```

### Anti-Patterns to Avoid
- **ACK before write:** Current code ACKs messages before S3 upload completes. A crash between ACK and flush loses events permanently. This is the most critical bug to fix.
- **Global rate limiter:** Current code uses a single `rate.Limiter` for all requests. Replace with per-API-key limiters after authentication is added.
- **Silencing errors with `_ = resource.Close()`:** Throughout the reaction engine. Log close errors at debug level minimum.
- **Unmarshal errors treated as retryable:** If a protobuf message cannot be unmarshaled, retrying won't help. These should go to DLQ via `msg.Term()`, not `msg.Nak()`.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bloom filter | Custom hash-set dedup | `bits-and-blooms/bloom/v3` | False positive tuning, serialization, well-tested at scale |
| Prometheus metrics | Custom `/metrics` endpoint | OTel + Prometheus exporter | Standard format, Grafana-compatible, histogram support |
| Token bucket rate limiting | Custom counter with time windows | `golang.org/x/time/rate` | Already in go.mod, handles burst correctly, thread-safe |
| Database migrations | Manual `CREATE TABLE` in code | `golang-migrate/migrate/v4` | Version tracking, up/down migrations, CI/CD friendly |
| Parquet file merging | Custom row-by-row copy | `parquet-go` MergeRowGroups | Handles schema validation, sorting, compression automatically |
| UUID generation | Custom ID schemes | `google/uuid` (UUIDv7) | Already in use, time-sortable, standard format |
| Structured logging | Custom JSON logger | `log/slog` (stdlib) | Already in use, supports JSON output, context-aware |
| API key comparison | `==` string compare | `crypto/subtle.ConstantTimeCompare` | Prevents timing side-channel attacks |

**Key insight:** Every component in this phase has a well-established Go library. The engineering challenge is integration and correct ordering (ACK-after-write), not building novel primitives.

## Common Pitfalls

### Pitfall 1: ACK Before Write (Data Loss)
**What goes wrong:** Messages are acknowledged to NATS before S3 upload completes. If the process crashes between ACK and flush, events are lost permanently.
**Why it happens:** The current code ACKs in the message processing loop (line 116, consumer.go) but events only enter a batch buffer. The flush to S3 happens later.
**How to avoid:** Retain NATS `jetstream.Msg` references in the batch. Only call `msg.Ack()` after `s3Client.Upload()` succeeds. On upload failure, call `msg.Nak()` to trigger redelivery.
**Warning signs:** Events "disappear" after warehouse-sink restart. NATS consumer sequence advances but S3 file count doesn't match.

### Pitfall 2: Bloom Filter False Positives Drop Real Events
**What goes wrong:** A bloom filter with too-small capacity or too-high FP rate silently drops unique events as "duplicates."
**Why it happens:** Under-sizing the bloom filter capacity or forgetting that false positives mean data loss.
**How to avoid:** Size the bloom filter for 2x expected volume. Use a low FP rate (0.01% = 0.0001). Log dropped events with `dedup_dropped` metric so false positive rate is observable. Consider a fallback: if the bloom filter becomes saturated, pass events through.
**Warning signs:** `dedup_dropped` metric grows faster than expected. Event counts in Trino are lower than NATS message counts.

### Pitfall 3: Compaction Race with Active Writers
**What goes wrong:** Compaction deletes small files while the warehouse-sink is actively writing new ones to the same partition.
**Why it happens:** Compaction lists files, reads them, writes merged file, deletes originals. If a new small file is written between list and delete, it could be missed or deleted.
**How to avoid:** Only compact partitions older than the current hour (or a configurable lag). Use S3 object versioning or atomic rename patterns. Never compact the "hot" partition being actively written to.
**Warning signs:** Missing events in Trino for recently-compacted partitions.

### Pitfall 4: Shutdown Timeout Too Short
**What goes wrong:** The warehouse-sink is killed before it can flush the last batch to S3.
**Why it happens:** Docker Compose default stop timeout is 10 seconds. If the batch has many events and S3 is slow, flush takes longer.
**How to avoid:** Set `stop_grace_period` in docker-compose.yml to at least 2x the flush interval. Default flush interval is 5m, so `stop_grace_period: 12m` is appropriate. Add a configurable shutdown timeout with a sensible default (60s).
**Warning signs:** Events lost on every deployment or restart.

### Pitfall 5: Per-Key Rate Limiter Memory Leak
**What goes wrong:** Creating a new `rate.Limiter` for every API key and never cleaning up fills memory.
**Why it happens:** `sync.Map.LoadOrStore` creates entries but doesn't expire them.
**How to avoid:** Run a periodic cleanup goroutine that removes limiters for keys not seen in the last N minutes. Or use a bounded LRU cache for limiters.
**Warning signs:** Steadily increasing memory usage on the HTTP server over time.

### Pitfall 6: SHA256 vs bcrypt for API Keys
**What goes wrong:** Using bcrypt (designed for passwords at ~100ms per hash) for per-request API key verification causes high latency and CPU.
**Why it happens:** Cargo-culting password hashing patterns for API keys.
**How to avoid:** API keys are long random strings (32+ bytes of entropy). SHA256 hashing is sufficient -- the key is already high-entropy, so brute-force is infeasible. Store `SHA256(key)` in PostgreSQL, compare with `subtle.ConstantTimeCompare` on each request.
**Warning signs:** API key validation middleware adds >10ms latency per request.

### Pitfall 7: NATS Consumer Name Collision
**What goes wrong:** Running multiple warehouse-sink replicas with the same consumer name causes message distribution (intended) but each replica builds its own partial batch, leading to many small files.
**Why it happens:** NATS distributes messages across consumers with the same durable name by design.
**How to avoid:** This is actually the desired behavior for horizontal scaling (R1.6). Ensure each replica can independently flush to S3. The compaction job (R1.3) will merge small files later. Set batch sizes and flush intervals to balance file size vs latency.
**Warning signs:** File count increases linearly with replica count (acceptable, compaction handles it).

## Code Examples

### API Key Authentication Module (R1.4)

```go
// internal/auth/internal/domain/apikey.go
package domain

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
)

type APIKey struct {
    ID        string
    AppID     string
    KeyHash   string // SHA256 hex
    Name      string
    Revoked   bool
    CreatedAt time.Time
}

// GenerateKey creates a new random API key and returns (plaintext, hash).
func GenerateKey() (string, string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", "", err
    }
    plaintext := hex.EncodeToString(bytes) // 64-char hex string
    hash := HashKey(plaintext)
    return plaintext, hash, nil
}

func HashKey(plaintext string) string {
    h := sha256.Sum256([]byte(plaintext))
    return hex.EncodeToString(h[:])
}
```

```sql
-- internal/auth/migrations/001_create_api_keys.up.sql
CREATE TABLE IF NOT EXISTS api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     TEXT NOT NULL,
    key_hash   TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL DEFAULT '',
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash) WHERE NOT revoked;
CREATE INDEX idx_api_keys_app_id ON api_keys(app_id);
```

### Auth Middleware (R1.4)

```go
// internal/auth/adapters.go
package auth

import (
    "crypto/sha256"
    "crypto/subtle"
    "encoding/hex"
    "net/http"
)

func (m *Module) AuthMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip auth for health/metrics endpoints
            if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
                next.ServeHTTP(w, r)
                return
            }

            apiKey := r.Header.Get("X-API-Key")
            if apiKey == "" {
                http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
                return
            }

            keyHash := hashKey(apiKey)
            key, err := m.svc.ValidateKey(r.Context(), keyHash)
            if err != nil || key == nil {
                http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
                return
            }

            // Add app_id to context for downstream use
            ctx := context.WithValue(r.Context(), AppIDKey, key.AppID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func hashKey(plaintext string) string {
    h := sha256.Sum256([]byte(plaintext))
    return hex.EncodeToString(h[:])
}
```

### Parquet Compaction Job (R1.3)

```go
// Source: parquet-go/parquet-go v0.27.0 MergeRowGroups API
// Compaction: list small files in partition, read row groups, merge, write, delete originals

func (c *CompactionService) CompactPartition(ctx context.Context, partition string) error {
    // List files in partition
    files, err := c.s3.ListObjects(ctx, partition)
    if err != nil {
        return err
    }

    // Filter to small files (< target size)
    var smallFiles []s3Object
    for _, f := range files {
        if f.Size < c.targetSize {
            smallFiles = append(smallFiles, f)
        }
    }
    if len(smallFiles) < 2 {
        return nil // nothing to compact
    }

    // Read all row groups from small files
    var rowGroups []parquet.RowGroup
    for _, f := range smallFiles {
        data, err := c.s3.Download(ctx, f.Key)
        if err != nil {
            return err
        }
        file, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
        if err != nil {
            return err
        }
        rowGroups = append(rowGroups, file.RowGroups()...)
    }

    // Merge row groups
    merged, err := parquet.MergeRowGroups(rowGroups)
    if err != nil {
        return err
    }

    // Write merged file
    var buf bytes.Buffer
    writer := parquet.NewGenericWriter[EventRow](&buf)
    if _, err := parquet.CopyRows(writer, merged.Rows()); err != nil {
        return err
    }
    if err := writer.Close(); err != nil {
        return err
    }

    // Upload merged file
    mergedKey := partition + "/compacted_" + uuid.New().String() + ".parquet"
    if err := c.s3.Upload(ctx, mergedKey, buf.Bytes()); err != nil {
        return err
    }

    // Delete original small files
    for _, f := range smallFiles {
        if err := c.s3.Delete(ctx, f.Key); err != nil {
            c.logger.Error("failed to delete compacted file", "key", f.Key, "error", err)
        }
    }

    return nil
}
```

### DLQ via NATS Advisory (R1.7)

```go
// Source: NATS docs - MaxDeliver advisory
// When MaxDeliver is exceeded, NATS publishes advisory to:
//   $JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.<STREAM>.<CONSUMER>

// Create a DLQ stream to capture failed messages
func (m *StreamManager) EnsureDLQStream(ctx context.Context) error {
    dlqCfg := jetstream.StreamConfig{
        Name:     "CAUSALITY_DLQ",
        Subjects: []string{"dlq.>"},
        Storage:  jetstream.FileStorage,
        MaxAge:   30 * 24 * time.Hour, // 30 days retention
    }
    _, err := m.js.CreateOrUpdateStream(ctx, dlqCfg)
    return err
}

// Subscribe to MaxDeliver advisory and republish to DLQ stream
func (c *DLQConsumer) Start(ctx context.Context) error {
    // Subscribe to advisory messages for max delivery exceeded
    advisorySubject := fmt.Sprintf(
        "$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.%s.%s",
        c.streamName, c.consumerName,
    )

    _, err := c.nc.Subscribe(advisorySubject, func(msg *nats.Msg) {
        // Parse advisory to get stream sequence
        var advisory struct {
            Stream   string `json:"stream"`
            Consumer string `json:"consumer"`
            StreamSeq uint64 `json:"stream_seq"`
        }
        if err := json.Unmarshal(msg.Data, &advisory); err != nil {
            c.logger.Error("failed to parse DLQ advisory", "error", err)
            return
        }

        // Fetch original message by sequence
        rawMsg, err := c.js.GetMsg(ctx, advisory.Stream, advisory.StreamSeq)
        if err != nil {
            c.logger.Error("failed to fetch DLQ message", "seq", advisory.StreamSeq)
            return
        }

        // Republish to DLQ stream
        c.js.Publish(ctx, "dlq."+rawMsg.Subject, rawMsg.Data)
        c.metrics.dlqDepth.Add(ctx, 1)
    })
    return err
}
```

### Go Backend SDK (R1.9)

```go
// sdk/go/causality.go
package causality

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
)

type Client struct {
    config    Config
    transport *httpTransport
    batch     []Event
    mu        sync.Mutex
    flushCh   chan struct{}
    doneCh    chan struct{}
}

type Config struct {
    APIKey        string
    Endpoint      string        // e.g., "http://localhost:8080"
    AppID         string
    BatchSize     int           // default: 50
    FlushInterval time.Duration // default: 30s
    MaxRetries    int           // default: 3
}

func New(cfg Config) *Client {
    if cfg.BatchSize == 0 {
        cfg.BatchSize = 50
    }
    if cfg.FlushInterval == 0 {
        cfg.FlushInterval = 30 * time.Second
    }
    if cfg.MaxRetries == 0 {
        cfg.MaxRetries = 3
    }

    c := &Client{
        config:    cfg,
        transport: newHTTPTransport(cfg),
        batch:     make([]Event, 0, cfg.BatchSize),
        flushCh:   make(chan struct{}, 1),
        doneCh:    make(chan struct{}),
    }
    go c.flushLoop()
    return c
}

// Track enqueues an event for sending. Non-blocking.
func (c *Client) Track(event Event) {
    event.IdempotencyKey = uuid.New().String()
    event.Timestamp = time.Now()

    c.mu.Lock()
    c.batch = append(c.batch, event)
    shouldFlush := len(c.batch) >= c.config.BatchSize
    c.mu.Unlock()

    if shouldFlush {
        select {
        case c.flushCh <- struct{}{}:
        default:
        }
    }
}

// Flush forces sending all queued events.
func (c *Client) Flush() error {
    return c.doFlush(context.Background())
}

// Close flushes remaining events and shuts down.
func (c *Client) Close() error {
    close(c.flushCh)
    <-c.doneCh
    return c.doFlush(context.Background())
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Push consumers (NATS) | Pull consumers | NATS server 2.9+ / nats.go v1.30+ | Better flow control, backpressure, horizontal scaling |
| `log` or `logrus` | `log/slog` (stdlib) | Go 1.21 (Aug 2023) | Structured logging in stdlib, no external dependency needed |
| Prometheus client directly | OTel API + Prometheus exporter | OTel Go SDK stable (2023+) | Vendor-neutral metrics, can switch backends without code changes |
| Manual Parquet writers | `parquet-go/parquet-go` generics API | parquet-go v0.20+ | `GenericWriter[T]` and `GenericReader[T]` for type-safe Parquet |
| `segmentio/parquet-go` | `parquet-go/parquet-go` | Fork in 2023 | Community fork after Segment archived original; active maintenance |

**Deprecated/outdated:**
- `segmentio/parquet-go`: Archived, use `parquet-go/parquet-go` (already in use)
- `go.opentelemetry.io/otel/exporter/metric/prometheus`: Old import path, use `go.opentelemetry.io/otel/exporters/prometheus`
- Push-based NATS consumers: Still work but pull consumers are recommended for new projects

## Open Questions

Things that could not be fully resolved:

1. **Hive Metastore Update After Compaction**
   - What we know: Compaction changes file layout in S3. Trino reads via Hive Metastore catalog.
   - What's unclear: Whether `CALL system.sync_partition_metadata()` in Trino automatically picks up file changes, or if specific Hive Metastore Thrift API calls are needed.
   - Recommendation: Test with `make trino-sync` after manual compaction. If automatic sync works, no Hive client needed. If not, add a Trino SQL call after compaction.

2. **EventEnvelope Idempotency Key Field**
   - What we know: R1.2 requires an idempotency key in every event. The proto `EventEnvelope` has `id` (UUID v7) and `correlation_id` but no explicit `idempotency_key` field.
   - What's unclear: Should we add a new proto field, or reuse the existing `id` field as the idempotency key?
   - Recommendation: Add an `idempotency_key` field to the proto. The `id` is server-generated (enriched in `service.go`), but the idempotency key should be SDK-generated and immutable. This keeps concerns separate.

3. **Compaction Target File Size**
   - What we know: R1.3 specifies 128-256 MB target. parquet-go MergeRowGroups can merge any number of row groups.
   - What's unclear: Actual file sizes in current production-like workloads. If events are small and volume is low, 128 MB may take days to accumulate.
   - Recommendation: Make target size configurable. Start with 128 MB as default. Measure actual file sizes with `make minio-size` before finalizing. Consider a minimum file count threshold (e.g., compact when partition has >10 files, regardless of total size).

4. **parquet-go Upgrade to v0.27.0**
   - What we know: Project uses v0.25.0, latest is v0.27.0. MergeRowGroups exists in both.
   - What's unclear: Whether the upgrade introduces breaking API changes (pre-v1 library).
   - Recommendation: Upgrade early in Phase 1 and run existing tests. Check changelog for breaking changes before upgrading.

## Sources

### Primary (HIGH confidence)
- [NATS JetStream Consumer Docs](https://docs.nats.io/using-nats/developer/develop_jetstream/consumers) - Pull consumer config, MaxDeliver, DLQ advisory pattern
- [NATS by Example - Pull Consumer (Go)](https://natsbyexample.com/examples/jetstream/pull-consumer/go) - Consume and Fetch patterns
- [bits-and-blooms/bloom v3.7.1](https://pkg.go.dev/github.com/bits-and-blooms/bloom/v3) - Bloom filter API, NewWithEstimates, Test, Add
- [parquet-go/parquet-go v0.27.0](https://pkg.go.dev/github.com/parquet-go/parquet-go) - MergeRowGroups, GenericWriter, OpenFile
- [OTel Prometheus Exporter v0.62.0](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/prometheus) - Exporter API, options
- [go.opentelemetry.io/otel v1.40.0](https://pkg.go.dev/go.opentelemetry.io/otel) - Metrics API, MeterProvider
- Existing codebase: `internal/warehouse/consumer.go`, `internal/gateway/`, `internal/nats/`, `internal/reaction/`

### Secondary (MEDIUM confidence)
- [OpenTelemetry Go Prometheus Example](https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/examples/prometheus) - OTel+Prometheus setup pattern
- [Synadia Blog - Job Queue with NATS and Go](https://www.synadia.com/blog/building-a-job-queue-with-nats-io-and-go) - DLQ pattern with NATS JetStream
- [Caio Ferreira - API Key Middleware in Go](https://caiorcferreira.github.io/post/golang-secure-api-key-middleware/) - SHA256 hashing, constant-time compare pattern
- [golang-migrate v4.18.2](https://github.com/golang-migrate/migrate) - Database migration tool
- [otelslog bridge](https://uptrace.dev/guides/opentelemetry-slog) - slog-to-OTel integration for trace_id injection

### Tertiary (LOW confidence)
- Compaction race condition handling: No authoritative source found for S3+Parquet compaction concurrent write safety. Recommendation (compact only cold partitions) is based on general distributed systems practice, not a specific library feature.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified via pkg.go.dev with current versions
- Architecture (ACK-after-write): HIGH - Pattern well-documented in NATS docs, directly addresses known codebase concern
- Architecture (module pattern): HIGH - Matches retcons backend pattern confirmed in codebase
- Pitfalls: HIGH - Derived from direct codebase analysis (CONCERNS.md) and NATS documentation
- Compaction: MEDIUM - MergeRowGroups API verified, but production compaction workflow (Hive Metastore sync) needs testing
- Go SDK: HIGH - Standard HTTP client + batching pattern, no novel components

**Research date:** 2026-02-05
**Valid until:** 2026-03-07 (30 days - stable domain, no rapidly changing components)
