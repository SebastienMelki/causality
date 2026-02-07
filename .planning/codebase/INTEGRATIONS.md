# External Integrations

**Analysis Date:** 2026-02-05

## APIs & External Services

**Event Streaming:**
- NATS JetStream 2.10+ - Reliable event pub/sub and queue management
  - SDK/Client: `github.com/nats-io/nats.go` v1.39.1
  - Connection: `NATS_URL` env var (default: `nats://localhost:4222`)
  - Subjects: `events.>`, `requests.>`, `responses.>`, `anomalies.>`
  - Stream name: `CAUSALITY_EVENTS`

**Webhook Delivery:**
- Custom HTTP webhook dispatcher in `internal/reaction/dispatcher.go`
  - Makes HTTP POST requests to user-configured webhook endpoints
  - Supports multiple auth types: none, basic auth, bearer token, HMAC-SHA256
  - Implements exponential backoff retry strategy with configurable intervals
  - Max retry attempts configurable (default checked from code: up to MaxDeliver)
  - Uses standard Go `net/http.Client` with 30s default timeout (configurable)

## Data Storage

**Databases:**
- **PostgreSQL 16+**
  - Connection: `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`, `DATABASE_SSL_MODE` env vars
  - Default database: `postgres:5432` with user `hive`, password `hive`
  - Client: `github.com/lib/pq` v1.10.9 (pure Go driver)
  - Databases:
    - `metastore` - Hive Metastore schema registry
    - `redash` - Redash configuration and query history
    - `reaction_engine` - Reaction engine rules, webhooks, anomaly configs, delivery tracking
  - Connection pooling: configurable via `DATABASE_MAX_OPEN_CONNS` (default 25), `DATABASE_MAX_IDLE_CONNS` (default 5), `DATABASE_CONN_MAX_LIFETIME` (default 5m)

**File Storage:**
- **MinIO / S3-compatible storage**
  - Provider: MinIO or AWS S3
  - Endpoint: `S3_ENDPOINT` env var (default: `http://minio:9000`)
  - Region: `S3_REGION` (default: `us-east-1`)
  - Bucket: `S3_BUCKET` (default: `causality-events`)
  - Credentials: `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` (default: `minioadmin/minioadmin`)
  - Client: AWS SDK v2 (`github.com/aws/aws-sdk-go-v2/service/s3`)
  - Path style: `S3_USE_PATH_STYLE=true` for MinIO compatibility
  - Format: Parquet files with Hive-style partitioning: `events/app_id={app}/year={yyyy}/month={mm}/day={dd}/hour={hh}/events_{uuid}.parquet`

**Caching:**
- Redis 7+ (required for Redash only, not for core services)
  - Used by: Redash background job queue
  - Connection: `redis://redis:6379/0` (hardcoded in docker-compose)

## Authentication & Identity

**Auth Provider:**
- Custom/None - No centralized auth system
- Webhook authentication handled by dispatcher with configurable auth methods:
  - **Basic Auth**: Username and password in Authorization header
  - **Bearer Token**: Token in Authorization header as `Bearer {token}`
  - **HMAC-SHA256**: Signature computed over request payload and sent as header (default: `X-Signature` or configurable)
  - **Custom Headers**: Arbitrary headers can be added to webhook requests via webhook config

**Reaction Engine Rule Evaluation:**
- Custom rule engine using CEL (Common Expression Language) via transitive dependency `cel.dev/expr`
- Rules stored as JSON in PostgreSQL with JSONPath condition evaluation
- No external auth services used

## Monitoring & Observability

**Error Tracking:**
- None detected - No Sentry, Bugsnag, or similar integration

**Logs:**
- Structured logging via Go `log/slog` standard library
- Log level configurable: `LOG_LEVEL` env var (debug, info, warn, error; default: info)
- Log format configurable: `LOG_FORMAT` env var (json, text; default: json)
- All services write to stdout
- Container logs accessible via `docker-compose logs`

**Health Checks:**
- NATS connection status checks: `/healthz` via NATS monitoring port 8222
- S3 bucket connectivity checks via `HeadBucket` operation
- PostgreSQL connection via `Ping`
- HTTP server health endpoints: `/health` (liveness) and `/ready` (readiness)

## CI/CD & Deployment

**Hosting:**
- Docker Compose (local development) - `docker-compose.yml`
- Docker containers for production - Multi-stage `Dockerfile`
- Target deployment: Kubernetes or container orchestration platforms

**CI Pipeline:**
- None detected in repository
- Manual build via `Makefile`: `make build`, `make docker-build`
- Lint support via golangci-lint: `make lint`

## Environment Configuration

**Required env vars (with defaults):**

**HTTP Server:**
- `HTTP_ADDR` (default: `:8080`)
- `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`
- `NATS_URL` (default: `nats://localhost:4222`)
- `LOG_LEVEL`, `LOG_FORMAT`

**Warehouse Sink:**
- `NATS_URL`
- `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_USE_PATH_STYLE`
- `BATCH_MAX_EVENTS`, `BATCH_FLUSH_INTERVAL`
- `PARQUET_COMPRESSION`, `PARQUET_ROW_GROUP_SIZE`

**Reaction Engine:**
- `NATS_URL`
- `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`, `DATABASE_SSL_MODE`
- `CONSUMER_NAME` (default: `analysis-engine`)
- `ENGINE_RULE_REFRESH_INTERVAL`, `DISPATCHER_WORKERS`, `DISPATCHER_POLL_INTERVAL`, `ANOMALY_CONFIG_REFRESH_INTERVAL`

**Secrets location:**
- Secrets passed as environment variables
- No `.env` file in codebase
- Docker Compose uses `./docker/redash/.env` for Redash configuration
- Production: Should use environment-specific secret management (Kubernetes Secrets, AWS Systems Manager Parameter Store, etc.)

## Webhooks & Callbacks

**Incoming:**
- HTTP POST `/v1/events/ingest` - Single event ingestion
- HTTP POST `/v1/events/batch` - Batch event ingestion
- Both endpoints accept JSON event objects with protobuf schema

**Outgoing:**
- Custom HTTP webhooks delivered by Reaction Engine Dispatcher
- Triggered when rules are matched or anomalies detected
- Endpoints configured in PostgreSQL `reaction_engine.webhooks` table
- Webhook delivery tracking in `reaction_engine.deliveries` table
- Payload format: JSON event data + rule/anomaly context
- Retry mechanism: Exponential backoff up to configurable max attempts
- Example webhook config fields:
  - `url` - Webhook endpoint
  - `method` - HTTP method (POST by default)
  - `headers` - Custom headers (JSON map)
  - `auth_type` - Authentication method (none, basic, bearer, hmac)
  - `auth_config` - Auth credentials (JSON, format varies by type)
  - `enabled` - Webhook activation flag

## Rate Limiting

**HTTP Gateway:**
- Rate limiting configurable via `RATE_LIMIT_ENABLED` (default: true)
- `RATE_LIMIT_REQUESTS_PER_SECOND` (default: 1000)
- `RATE_LIMIT_BURST_SIZE` (default: 2000)
- Implemented in `internal/gateway/middleware.go`

## CORS Configuration

**Settings:**
- `CORS_ALLOWED_ORIGINS` (default: `*`)
- `CORS_ALLOWED_METHODS` (default: GET, POST, PUT, DELETE, OPTIONS)
- `CORS_ALLOWED_HEADERS` (default: Accept, Authorization, Content-Type, X-Request-ID, X-Correlation-ID)
- `CORS_EXPOSED_HEADERS` (default: X-Request-ID)
- `CORS_ALLOW_CREDENTIALS` (default: false)
- `CORS_MAX_AGE` (default: 86400 seconds / 24 hours)

## Protocol Buffer Dependencies

**buf.build Dependencies:**
- `buf.build/bufbuild/protovalidate` - Proto validation generation
- `buf.build/sebmelki/sebuf` - HTTP handler and OpenAPI generation

**Generation:**
- Protoc plugins: `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-go-http`, `protoc-gen-openapiv3`
- Generated code location: `pkg/proto/`
- Proto sources location: `proto/`
- Protobuf generation managed by buf: `make buf-generate`

---

*Integration audit: 2026-02-05*
