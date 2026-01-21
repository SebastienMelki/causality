# Causality

A behavioral analysis system that detects application modifications by analyzing event patterns and identifying anomalies from expected behavior.

## Overview

Causality collects events from mobile and web applications, stores them in a data warehouse, and enables SQL-based analytics for behavioral pattern analysis and anomaly detection.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   Mobile Apps   │     │    Web Apps     │
│  (iOS/Android)  │     │   (Browser)     │
└────────┬────────┘     └────────┬────────┘
         │                       │
         └───────────┬───────────┘
                     │
              ┌──────▼──────┐
              │ HTTP Server │ :8080
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │    NATS     │ JetStream
              └──────┬──────┘
                     │
         ┌───────────┴───────────┐
         │                       │
  ┌──────▼──────┐         ┌──────▼──────┐
  │  Warehouse  │         │  Reaction   │
  │    Sink     │         │   Engine    │
  └──────┬──────┘         └──────┬──────┘
         │                       │
  ┌──────▼──────┐         ┌──────▼──────┐
  │   MinIO     │         │  Webhooks/  │
  │  (Parquet)  │         │   Alerts    │
  └──────┬──────┘         └─────────────┘
         │
  ┌──────▼──────┐
  │    Trino    │ SQL Analytics
  └──────┬──────┘
         │
  ┌──────▼──────┐
  │   Redash    │ Visualization
  └─────────────┘
```

### Components

- **HTTP Server**: RESTful API for event ingestion (`/v1/events/ingest`, `/v1/events/batch`)
- **Admin UI**: Web-based configuration management (`/admin`)
- **NATS JetStream**: Event streaming and reliable delivery
- **Warehouse Sink**: Consumes events, writes Parquet files to S3
- **Reaction Engine**: Rule evaluation, anomaly detection, webhook delivery
- **MinIO**: S3-compatible object storage for event data
- **Hive Metastore**: Schema registry for Trino
- **Trino**: SQL query engine for analytics on Parquet files
- **Redash**: Data visualization and dashboards

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for development)
- Make

### Start the Environment

```bash
# Start all services (clean)
make dev

# Or start without cleaning existing data
make docker-up
```

This starts:
- HTTP Server: http://localhost:8080
- Admin UI: http://localhost:8080/admin
- NATS Monitoring: http://localhost:8222
- MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
- Trino: http://localhost:8085
- Redash: http://localhost:5050 (admin@causality.local/admin123)

### Send Test Events

```bash
# Send a single event
make test-event

# Send a batch of events
make test-batch

# Send 100 uniform events
make test-load

# Send random events with variation (better for graphs)
make test-random
```

### Query Events

```bash
# Open Trino CLI
make trino-cli

# Sync partitions and count events
make trino-sync
make trino-count

# View event statistics
make trino-stats
```

Or use Redash at http://localhost:5050 with SQL:

```sql
SELECT
  event_type,
  count(*) AS event_count
FROM hive.causality.events
GROUP BY event_type
ORDER BY event_count DESC
```

### Admin UI

Access the Admin UI at http://localhost:8080/admin to manage:

- **Rules**: Create event matching rules with JSONPath conditions
- **Webhooks**: Configure webhook endpoints with authentication
- **Anomaly Detection**: Set up threshold, rate, or count-based anomaly detection
- **Event Browser**: Query and browse events from Trino
- **Custom Event Types**: Define custom event schemas

## API

### Ingest Single Event

```bash
curl -X POST http://localhost:8080/v1/events/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "event": {
      "appId": "my-app",
      "deviceId": "device-001",
      "screenView": {"screenName": "HomeScreen"}
    }
  }'
```

### Ingest Batch

```bash
curl -X POST http://localhost:8080/v1/events/batch \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {"appId": "my-app", "deviceId": "d1", "screenView": {"screenName": "Home"}},
      {"appId": "my-app", "deviceId": "d1", "buttonTap": {"buttonId": "login", "screenName": "Home"}}
    ]
  }'
```

### Event Types

- `screenView`: Screen/page views
- `screenExit`: Screen exit with duration
- `buttonTap`: Button/UI interactions
- `userLogin` / `userLogout`: Authentication events
- `productView` / `addToCart` / `purchaseComplete`: E-commerce events
- `appStart` / `appBackground` / `appForeground`: Lifecycle events
- `networkChange`: Connectivity changes
- `customEvent`: Custom events with arbitrary parameters

## Project Structure

```
causality/
├── cmd/
│   ├── server/           # HTTP server
│   ├── warehouse-sink/   # NATS consumer → Parquet → S3
│   └── reaction-engine/  # Rule evaluation and anomaly detection
├── internal/
│   ├── admin/            # Admin UI (templ + HTMX)
│   ├── events/           # Shared event categorization
│   ├── gateway/          # HTTP routing and handlers
│   ├── nats/             # JetStream client
│   ├── warehouse/        # Parquet writer and S3 upload
│   └── reaction/         # Rule engine, anomaly detection, webhooks
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

## Development

### Build Locally

```bash
# Build server binary
make build-server

# Build warehouse sink
make build-sink

# Build reaction engine
make build-reaction

# Build all
make build
```

### Run Tests

```bash
make test
```

### Useful Commands

```bash
make help           # Show all available commands
make docker-logs    # Tail all service logs
make docker-ps      # Show running containers
make minio-ls       # List objects in MinIO
make nats-info      # Show NATS server info
```

## Configuration

### Environment Variables

**HTTP Server:**
- `HTTP_ADDR`: Listen address (default: `:8080`)
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)
- `ADMIN_ENABLED`: Enable admin UI (default: `true`)
- `DATABASE_HOST` / `DATABASE_PORT`: PostgreSQL for admin UI
- `TRINO_HOST` / `TRINO_PORT`: Trino connection for event browser

**Warehouse Sink:**
- `NATS_URL`: NATS server URL
- `S3_ENDPOINT`: S3/MinIO endpoint
- `S3_BUCKET`: Bucket name (default: `causality-events`)
- `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY`: Credentials
- `BATCH_MAX_EVENTS`: Events per Parquet file (default: `1000`)
- `BATCH_FLUSH_INTERVAL`: Max time before flush (default: `30s`)

**Reaction Engine:**
- `NATS_URL`: NATS server URL
- `DATABASE_HOST` / `DATABASE_PORT`: PostgreSQL connection
- `DATABASE_USER` / `DATABASE_PASSWORD`: PostgreSQL credentials
- `DATABASE_NAME`: Database name (default: `reaction_engine`)
- `ENGINE_RULE_REFRESH_INTERVAL`: Rule cache refresh interval (default: `30s`)
- `DISPATCHER_WORKERS`: Webhook delivery workers (default: `5`)

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
