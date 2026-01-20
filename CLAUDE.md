# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Causality is a behavioral analysis system that collects events from mobile (iOS/Android) and web applications, stores them in a data warehouse, and enables SQL-based analytics for behavioral pattern analysis and anomaly detection.

**Core Stack:**
- **Go 1.24+** - Backend services
- **Protocol Buffers** - Event definitions and API contracts
- **NATS JetStream** - Event streaming and reliable delivery
- **MinIO** - S3-compatible object storage
- **Apache Parquet** - Columnar storage format
- **Trino** - SQL query engine
- **Redash** - Data visualization

Module: `github.com/SebastienMelki/causality`

## Quick Start

```bash
# Start complete development environment (clean)
make dev

# Or start without cleaning existing data
make docker-up

# Send test events
make test-event      # Single event
make test-batch      # Batch of events
make test-random     # Random varied events (for graphs)

# Query data
make trino-cli       # Open Trino CLI
make trino-sync      # Sync partitions from S3
make trino-stats     # View event statistics
```

## Common Commands

### Development Environment
- **Start environment (clean)**: `make dev`
- **Start environment**: `make docker-up`
- **Stop environment**: `make docker-down`
- **View logs**: `make docker-logs`
- **Rebuild Go services**: `make dev-rebuild`
- **Clean everything**: `make docker-clean`

### Building
- **Build all**: `make build`
- **Build server**: `make build-server`
- **Build warehouse sink**: `make build-sink`

### Testing
- **Run tests**: `make test`
- **Run with coverage**: `make test-coverage`
- **Send test event**: `make test-event`
- **Send batch**: `make test-batch`
- **Load test (100 events)**: `make test-load`
- **Random events (for graphs)**: `make test-random`

### Trino & Data
- **Open Trino CLI**: `make trino-cli`
- **Sync partitions**: `make trino-sync`
- **Count events**: `make trino-count`
- **Event statistics**: `make trino-stats`
- **Custom query**: `make trino-query SQL="SELECT * FROM hive.causality.events LIMIT 10"`

### Protocol Buffers
- **Generate code**: `make buf-generate` or `make generate`
- **Lint protos**: `make buf-lint`

### Code Quality
- **Format code**: `make fmt`
- **Run linter**: `make lint`
- **Run linter with auto-fix**: `make lint-fix`
- **Run vet**: `make vet`
- **All checks**: `make check`

### NATS & MinIO
- **NATS info**: `make nats-info`
- **NATS streams**: `make nats-streams`
- **MinIO list**: `make minio-ls`
- **MinIO size**: `make minio-size`

## Project Structure

```
causality/
├── cmd/
│   ├── server/           # HTTP server for event ingestion
│   └── warehouse-sink/   # NATS consumer → Parquet → S3
├── internal/
│   ├── events/           # Shared event categorization
│   ├── gateway/          # HTTP routing and handlers
│   ├── nats/             # JetStream client
│   └── warehouse/        # Parquet writer and S3 upload
├── pkg/proto/            # Generated protobuf code
├── proto/                # Protocol buffer definitions
├── docker/
│   ├── hive/             # Hive Metastore config
│   ├── trino/            # Trino config
│   ├── redash/           # Redash setup scripts
│   └── postgres/         # PostgreSQL init
├── sql/                  # Trino table definitions
├── docker-compose.yml    # Development environment
├── Dockerfile            # Multi-stage build
└── Makefile              # Development commands
```

## Architecture Overview

### Data Flow
```
Mobile/Web Apps → HTTP Server → NATS JetStream → Warehouse Sink → MinIO (Parquet)
                                                                        ↓
                                                              Hive Metastore
                                                                        ↓
                                                                    Trino → Redash
```

1. **Event Ingestion**: Apps POST events to HTTP server
2. **Streaming**: Events published to NATS JetStream
3. **Batching**: Warehouse sink batches events
4. **Storage**: Parquet files written to MinIO (S3)
5. **Catalog**: Hive Metastore tracks table schema
6. **Querying**: Trino queries Parquet files via Hive connector
7. **Visualization**: Redash dashboards

### Key Components

#### HTTP Server (`cmd/server`)
- RESTful API for event ingestion
- Endpoints: `/v1/events/ingest`, `/v1/events/batch`
- Health checks: `/health`, `/ready`
- Publishes events to NATS

#### Warehouse Sink (`cmd/warehouse-sink`)
- Consumes events from NATS JetStream
- Batches events (configurable size/interval)
- Writes Parquet files to S3
- Hive-style partitioning: `app_id/year/month/day/hour`

#### Trino Configuration
- Uses Hive connector with S3 storage
- Supports pyhive (Presto protocol) for Redash
- Config: `docker/trino/etc/`

#### Redash Setup
- Auto-creates admin user on startup
- Auto-configures Trino data source
- Scripts: `docker/redash/`

## Service Ports

| Service | Port | URL |
|---------|------|-----|
| HTTP Server | 8080 | http://localhost:8080 |
| NATS | 4222 | nats://localhost:4222 |
| NATS Monitoring | 8222 | http://localhost:8222 |
| MinIO API | 9000 | http://localhost:9000 |
| MinIO Console | 9001 | http://localhost:9001 |
| Trino | 8085 | http://localhost:8085 |
| Redash | 5050 | http://localhost:5050 |
| Hive Metastore | 9083 | thrift://localhost:9083 |
| PostgreSQL | 5432 | postgres://localhost:5432 |

## Event Types

- `screenView`: Screen/page views
- `screenExit`: Screen exit with duration
- `buttonTap`: Button/UI interactions
- `userLogin` / `userLogout`: Authentication events
- `productView` / `addToCart` / `purchaseComplete`: E-commerce events
- `appStart` / `appBackground` / `appForeground`: Lifecycle events
- `networkChange`: Connectivity changes
- `customEvent`: Custom events with arbitrary parameters

## Development Guidelines

### Code Organization
- Use `internal/` for non-exported application logic
- Keep protocol definitions in `proto/` directory
- Place generated code in `pkg/proto/`
- Docker configs in `docker/` directory
- Write comprehensive tests alongside implementation

### Testing Strategy
- Unit tests for all business logic
- Integration tests for HTTP API endpoints
- E2E tests with Docker environment
- Use `make test-random` for realistic load testing

### Common Patterns
- Use context.Context for cancellation
- Implement graceful shutdown for servers
- Event batching for efficient writes
- Hive-style partitioning for query performance