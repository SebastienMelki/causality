# Testing Patterns

**Analysis Date:** 2026-02-05

## Test Framework

**Runner:**
- Go's built-in `testing` package (standard library)
- No external test framework required

**Assertion Library:**
- Standard Go: manual comparisons using `if` statements
- No assertion library dependencies (testify not used)

**Run Commands:**
```bash
make test              # Run all unit tests (go test -v ./...)
make test-coverage     # Run tests with coverage (generates coverage/coverage.html)
make test-load         # Load test: 100 events (integration)
make test-random       # Random varied events for realistic testing
```

**Coverage:**
```bash
go test -coverprofile=coverage/coverage.out ./...
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
```

## Test File Organization

**Location:**
- Co-located in same package as implementation
- Same directory, `*_test.go` suffix

Example locations:
- Implementation: `internal/nats/publisher.go`
- Tests: `internal/nats/publisher_test.go`

**Naming:**
- File: `{source}_test.go`
- Functions: `Test{FunctionName}` prefix

Examples:
- `TestDeriveSubject` tests `publisher.deriveSubject()`
- `TestEventService_IngestEvent` tests `EventService.IngestEvent()`
- `TestEventRowFromProto` tests the `EventRowFromProto()` function

**Structure:**
```
{package}_test.go
├── Test functions
├── Helper functions (lowercase, unexported)
├── Mock implementations
└── Test fixtures
```

## Test Structure

**Suite Organization:**

Tests use table-driven pattern (subtests):

```go
func TestDeriveSubject(t *testing.T) {
	tests := []struct {
		name     string
		event    *pb.EventEnvelope
		expected string
	}{
		{
			name: "screen view event",
			event: &pb.EventEnvelope{
				AppId:    "myapp",
				DeviceId: "device123",
				Payload: &pb.EventEnvelope_ScreenView{
					ScreenView: &pb.ScreenView{
						ScreenName: "home",
					},
				},
			},
			expected: "events.myapp.screen.view",
		},
		// ... more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := publisher.DeriveSubjectForTest(tt.event)
			if result != tt.expected {
				t.Errorf("DeriveSubject() = %q, want %q", result, tt.expected)
			}
		})
	}
}
```

**Test case structure:**
- `name`: Human-readable test description
- `{input fields}`: Test inputs (event, request, config, etc.)
- `expected` or `want{Field}`: Expected outputs
- `shouldFail` or `wantErr`: Whether error is expected

**Patterns:**

1. **Setup:** Create test table with test cases
2. **Loop:** Iterate with `t.Run(tt.name, func(t *testing.T) {...})`
3. **Arrange:** Prepare test inputs (already in table)
4. **Act:** Call function under test
5. **Assert:** Compare results with expected values

From `internal/gateway/service_test.go`:
```go
func TestEventService_IngestEvent(t *testing.T) {
	tests := []struct {
		name       string
		request    *pb.IngestEventRequest
		shouldFail bool
		wantErr    bool
	}{
		{
			name: "valid event without ID",
			request: &pb.IngestEventRequest{
				Event: &pb.EventEnvelope{
					AppId:    "testapp",
					DeviceId: "device123",
					Payload: &pb.EventEnvelope_ScreenView{
						ScreenView: &pb.ScreenView{
							ScreenName: "home",
						},
					},
				},
			},
			shouldFail: false,
			wantErr:    false,
		},
		// ... more cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test logic here
		})
	}
}
```

## Mocking

**Framework:** Manual mocking (no testify/mock or other library)

**Patterns:**

Create mock structs that implement the interface:

From `internal/gateway/service_test.go`:
```go
type mockPublisher struct {
	publishedEvents []*pb.EventEnvelope
	shouldFail      bool
}

func (m *mockPublisher) PublishEvent(ctx context.Context, event *pb.EventEnvelope) error {
	if m.shouldFail {
		return context.DeadlineExceeded
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}
```

**What to Mock:**
- External dependencies (NATS publisher, S3 client, database)
- Interfaces, not concrete types
- Services called by the code under test

**What NOT to Mock:**
- Pure functions (no side effects)
- Data structures/value types
- Protocol buffer messages (construct real instances)
- Standard library types

## Fixtures and Factories

**Test Data:**

Protocol buffer messages are constructed directly in test tables:

```go
event: &pb.EventEnvelope{
	Id:          "01234567-89ab-cdef-0123-456789abcdef",
	AppId:       "testapp",
	DeviceId:    "device123",
	TimestampMs: time.Now().UnixMilli(),
	Payload: &pb.EventEnvelope_UserLogin{
		UserLogin: &pb.UserLogin{
			UserId: "user456",
			Method: "email",
		},
	},
}
```

**Location:**
- Fixtures are inline in test tables
- No separate fixtures directory (tests are self-contained)
- Test data is built up in the test struct directly

**Builder patterns:** Not used; construct protobuf objects directly with all fields.

## Coverage

**Requirements:** Not enforced in CI/linting (no coverage gate configured)

**View Coverage:**
```bash
make test-coverage          # Generate and open HTML report
# Opens: coverage/coverage.html
```

The Makefile generates:
- `coverage/coverage.out`: Raw coverage data
- `coverage/coverage.html`: Interactive coverage report

## Test Types

**Unit Tests:**
- Scope: Single function or method
- Approach: Table-driven with mocked dependencies
- Location: `*_test.go` files co-located with source
- Examples:
  - `TestDeriveSubject`: Tests `Publisher.deriveSubject()`
  - `TestEventService_IngestEvent`: Tests service business logic
  - `TestEventRowFromProto`: Tests data transformation
  - `TestParquetWriter_Write`: Tests Parquet encoding

**Integration Tests:**
- Scope: Multiple components working together
- Approach: Use Docker environment
- Commands: `make test-e2e` (tagged with `e2e` build tag)
- Location: `tests/e2e/...` (to be created)
- Not yet extensively used; framework in place

**E2E Tests:**
- Framework: Go testing with `e2e` build tag
- Run: `make test-e2e`
- Requires: `docker-compose up` running
- Coverage: Full system integration (server → NATS → warehouse sink)

**Manual Testing:**
```bash
make test-event           # Send single test event
make test-batch           # Send batch of events
make test-random          # Send varied random events
make test-load            # Send 100 events
make trino-stats          # View event statistics in Trino
```

## Common Patterns

**Table-Driven Tests:**

Used throughout the codebase. Structure:
1. Define `tests` slice with struct containing test case data
2. Loop through with `t.Run()` for subtest isolation
3. Each test case is independent and can be run in any order

From `internal/warehouse/parquet_test.go`:
```go
tests := []struct {
	compression string
}{
	{"snappy"},
	{"gzip"},
	{"zstd"},
	{"none"},
}

for _, tt := range tests {
	t.Run(tt.compression, func(t *testing.T) {
		writer := NewParquetWriter(ParquetConfig{
			Compression: tt.compression,
		})

		data, err := writer.Write(rows)
		if err != nil {
			t.Fatalf("Write() with compression %q error = %v", tt.compression, err)
		}
		// assertions...
	})
}
```

**Async Testing:**

Not heavily used in current test suite. Go's `testing.T` is single-threaded per test. Use `t.Run()` for parallel test isolation.

Pattern for testing concurrent operations:
```go
// Goroutines within a test are allowed
go func() {
	// async operation
}()

// Wait for completion with channels or WaitGroup
```

**Error Testing:**

Expected errors checked with boolean flags:

```go
{
	name:       "nil event",
	request: &pb.IngestEventRequest{
		Event: nil,
	},
	shouldFail: false,
	wantErr:    true,
}

// In test:
if (err != nil) != tt.wantErr {
	t.Errorf("wantErr %v, got error %v", tt.wantErr, err != nil)
}
```

**Nil/Default Value Testing:**

From `internal/gateway/service_test.go`:
```go
{
	name: "nil event",
	request: &pb.IngestEventRequest{
		Event: nil,
	},
	wantErr: true,
}
```

**Enrichment Logic Testing:**

Tests verify that service layer enriches data correctly:

From `internal/gateway/service_test.go`:
```go
func TestEnrichEnvelope(t *testing.T) {
	tests := []struct {
		name             string
		event            *pb.EventEnvelope
		wantIDGenerated  bool
		wantTSGenerated  bool
	}{
		{
			name: "empty event gets enriched",
			event: &pb.EventEnvelope{
				AppId:    "test",
				DeviceId: "dev",
			},
			wantIDGenerated: true,
			wantTSGenerated: true,
		},
		// ... more cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate enrichment logic
			if tt.event.Id == "" {
				tt.event.Id = "new-generated-id"
			}
			if tt.event.TimestampMs == 0 {
				tt.event.TimestampMs = time.Now().UnixMilli()
			}

			// Check that enrichment occurred as expected
			if tt.wantIDGenerated {
				if tt.event.Id == originalID || tt.event.Id == "" {
					t.Error("Expected ID to be generated")
				}
			}
		})
	}
}
```

## Linting Exceptions for Tests

Tests are excluded from certain linters (in `.golangci.yml`):
- `bodyclose`: Not enforced (cleanup in tests is different)
- `cyclop`: High complexity allowed
- `dupl`: Duplication allowed (test fixtures can be repetitive)
- `err113`: Error wrapping rules relaxed
- `errcheck`: Some error checks skipped
- `funlen`: Function length limits relaxed
- `gocognit`: Cognitive complexity limits relaxed
- `goconst`: Magic constant rules relaxed
- `gosec`: Security rules relaxed
- `noctx`: Context requirement relaxed
- `revive`: Many revive rules skipped

This allows tests to be more pragmatic while keeping main code clean.

## Test Coverage Focus

**Well-tested areas:**
- Event subject derivation (`TestDeriveSubject`): 14 test cases
- Event category/type mapping: Comprehensive coverage
- Parquet row transformation (`TestEventRowFromProto`): 2+ scenarios
- Enrichment logic (ID generation, timestamp): Multiple cases
- Batch operations: Partial failure scenarios

**Recommended additional testing:**
- S3 upload retry logic
- NATS connection/reconnection behavior
- Webhook delivery retry logic (in reaction engine)
- Error scenarios in warehouse sink
- Database transaction handling

---

*Testing analysis: 2026-02-05*
