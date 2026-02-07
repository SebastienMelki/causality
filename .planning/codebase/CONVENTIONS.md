# Coding Conventions

**Analysis Date:** 2026-02-05

## Naming Patterns

**Files:**
- Package files use lowercase with underscores: `publisher.go`, `warehouse_sink.go`, `parquet_test.go`
- Main entry points: `cmd/*/main.go`
- Test files: `*_test.go` suffix in same package as implementation

**Functions:**
- Exported functions use PascalCase: `NewPublisher()`, `PublishEvent()`, `IngestEvent()`
- Private functions use camelCase: `deriveSubject()`, `enrichEnvelope()`, `serializePayload()`
- Factory functions prefix with `New`: `NewClient()`, `NewServer()`, `NewPublisher()`

**Variables:**
- Package-level (exported): PascalCase: `Publisher`, `Client`, `EventService`
- Local variables and parameters: camelCase: `ctx`, `event`, `publishedEvents`
- Constants: ALL_CAPS with underscores: `CAUSALITY_EVENTS` (stream name), `NATS_URL` (config key)
- Acronyms preserved as-is: `ID`, `AppID`, `DeviceID`, `EventEnvelope`, `OSVersion`, `JSONPath`

**Types:**
- Struct names: PascalCase, descriptive: `EventEnvelope`, `Publisher`, `EventService`, `DeviceContext`
- Interface names: Often use single letter receiver or descriptive name
- Sentinel error names: `Err` prefix followed by description: `ErrNotConnected`, `ErrPartialPublish`, `ErrRuleNotFound`

**Config structs:**
- Use struct tags for environment variables: `env:"LOG_LEVEL" envDefault:"info"`
- Use struct tags for Parquet serialization: `parquet:"id,snappy,dict"`
- Prefix matching in env parsing: `envPrefix:""`

## Code Style

**Formatting:**
- Use `go fmt` for automatic formatting
- Command: `make fmt` runs `go fmt ./...`

**Linting:**
- Tool: `golangci-lint` v2
- Config: `.golangci.yml`
- Run: `make lint` or `make lint-fix` (auto-fix)
- Max function length: 100 lines with 50 statements (funlen rule)
- Cyclomatic complexity: 30 max (cyclop), 15 package average
- Cognitive complexity: 50 max (gocognit)
- Nested if complexity: 4 min (nestif)
- Naked returns: Not allowed (nakedret)

**Key enabled linters:**
- `errcheck`: Check unchecked errors, including type assertions
- `govet`: Including shadow variable detection
- `staticcheck`: All checks enabled
- `revive`: Multiple style rules (see `.golangci.yml` for full list)
- `gosec`: Security linting (excluding G204, G304 for subprocess/file operations)

**Import Organization:**

Order imports as follows:
1. Standard library (e.g., `"context"`, `"fmt"`, `"log/slog"`)
2. External packages (e.g., `"github.com/nats-io/nats.go"`)
3. Local application packages (e.g., `"github.com/SebastienMelki/causality/internal/..."`)

Separate groups with blank lines.

Example from `internal/nats/publisher.go`:
```go
import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	"github.com/SebastienMelki/causality/internal/events"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)
```

**Path Aliases:**
- Local packages: No alias required
- Generated protobufs: Aliased as `pb`: `pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"`
- goimports local prefix: `github.com/SebastienMelki/causality`

## Error Handling

**Patterns:**

Use sentinel errors (errors.New) for well-known error conditions:
- Located in `errors.go` file per package
- Named with `Err` prefix
- Declared as package-level var in error files

Example from `internal/nats/errors.go`:
```go
var (
	ErrNotConnected     = errors.New("NATS is not connected")
	ErrPartialPublish   = errors.New("failed to publish some events")
)
```

**Error wrapping:**
- Use `fmt.Errorf("failed to ...: %w", err)` for context wrapping
- Never use `%v` with errors; use `%w` for wrapping chains

Example from `internal/nats/publisher.go`:
```go
if err != nil {
	return fmt.Errorf("failed to marshal event: %w", err)
}
```

**Error propagation in HTTP handlers:**
- Return error from service layer
- Handler converts to HTTP status codes
- Log errors at point of origin with context

Example from `internal/gateway/service.go`:
```go
if err := s.publisher.PublishEvent(ctx, req.GetEvent()); err != nil {
	s.logger.Error("failed to publish event",
		"event_id", req.GetEvent().GetId(),
		"error", err,
	)
	return nil, fmt.Errorf("failed to publish event: %w", err)
}
```

**Partial failures:**
- Continue processing remaining items in batch
- Return partial result count + error
- Example: `PublishEventBatch` returns `(int, error)` with count of successful publishes

## Logging

**Framework:** `log/slog` (structured logging, Go 1.21+)

**Default configuration:**
- Text format in development: `setupLogger()` in main packages
- JSON format in production
- Configurable via `LOG_LEVEL` and `LOG_FORMAT` env vars

**Patterns:**

Use structured fields, not string concatenation:
```go
// CORRECT
p.logger.Debug("event published",
	"event_id", event.GetId(),
	"subject", subject,
	"stream", ack.Stream,
	"sequence", ack.Sequence,
)

// WRONG
p.logger.Debug(fmt.Sprintf("event %s published to %s", event.GetId(), subject))
```

**Log levels:**
- `Debug`: Detailed operational info (subject derivation, marshaling)
- `Info`: Important lifecycle events (service startup, connection established)
- `Warn`: Recoverable issues (disconnects, reconnects)
- `Error`: Errors requiring attention (publish failures, validation errors)

**Component context:**
- Add "component" field at logger creation: `logger.With("component", "publisher")`
- This automatically propagates to all logs from that logger

Example from `internal/nats/publisher.go`:
```go
logger     *slog.Logger

func NewPublisher(js jetstream.JetStream, streamName string, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{
		js:         js,
		streamName: streamName,
		logger:     logger.With("component", "publisher"),
	}
}
```

**Never log sensitive data:** No passwords, tokens, or PII in logs.

## Comments

**When to Comment:**

- **Public types/functions:** Always include comment above export
  - One-liner summary of purpose
  - Additional detail if complex

Example from `internal/warehouse/parquet.go`:
```go
// EventRow is the flattened structure for Parquet storage.
// This schema is optimized for analytics queries via Hive/Athena.
type EventRow struct {
```

- **Private functions:** Comment if behavior is non-obvious
- **Algorithm explanation:** Comment complex logic
- **TODO/FIXME:** Add prefix comment when needed (rare in this codebase)

**JSDoc/Doc Comments:**
- Follow Go convention: comment appears immediately before declaration
- One-line summary starting with exported name/verb
- Multi-line: additional sentences on following lines (first-line is summary)

Example from `internal/nats/publisher.go`:
```go
// PublishEvent publishes a single event to the appropriate NATS subject.
func (p *Publisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {

// PublishEventBatch publishes multiple events to NATS.
// Returns the number of successfully published events and any error.
func (p *Publisher) PublishEventBatch(ctx context.Context, events []*pb.EventEnvelope) (int, error) {
```

- **Code comments:** Explain "why", not "what"
- Inline comments for gotchas and non-obvious behavior

Example from `internal/nats/publisher.go`:
```go
// Sanitize app_id for subject (replace dots with underscores)
appID := strings.ReplaceAll(event.GetAppId(), ".", "_")
```

Example with gosec nolint comment:
```go
result.Index = int32(i) //nolint:gosec // Index is bounded by batch size which is well under int32 max.
```

## Function Design

**Size:**
- Aim for < 100 lines (funlen limit: 100)
- Target < 50 statements
- Single responsibility

**Parameters:**
- Use `context.Context` as first parameter for cancellation/timeouts
- Group related parameters (avoid too many scalar params)
- Receiver for methods: single letter `(p *Publisher)`, `(s *EventService)`, `(c *Client)`

Example from `internal/nats/publisher.go`:
```go
func (p *Publisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {
```

**Return Values:**
- Return error as last value
- For functions returning multiple items: `(result, error)`
- For functions returning count + error: `(count int, err error)`

Example from `internal/nats/publisher.go`:
```go
// Returns count and error
func (p *Publisher) PublishEventBatch(ctx context.Context, events []*pb.EventEnvelope) (int, error) {

// Returns single value and error
func (p *Publisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {
```

**Named returns:** Avoid named returns for simple functions (no names in `PublishEvent`), use for functions with multiple return values where context matters.

## Module Design

**Exports:**
- Package-level `New*` constructor functions for main types
- Document all exported symbols
- Small, focused exports (no God objects)

**Barrel Files:**
- Not used in this codebase
- Each module has explicit imports from subpackages

**Package organization:**
- `internal/`: Application-specific logic
- `pkg/proto/`: Generated protobuf code (auto-generated, excluded from linting)
- `cmd/`: Entry points for executables

**Dependencies:**
- Minimal external dependencies
- Protocol Buffers for contracts between services
- slog for logging (standard library)
- NATS client for messaging
- Parquet-go for columnar storage
- AWS SDK v2 for S3 operations

---

*Convention analysis: 2026-02-05*
