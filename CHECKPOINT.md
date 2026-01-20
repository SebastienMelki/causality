# Checkpoint - Causality Implementation Status

## What's Done

### 1. Core Event Processing ✅
- **Proto definitions**: `proto/causality/v1/events.proto` and `service.proto`
- **Sebuf HTTP handlers**: Generated in `pkg/proto/causality/v1/service_http*.pb.go`
- **OpenAPI spec**: Generated in `api/openapi/EventService.openapi.yaml`
- **Event gateway**: `internal/gateway/` - HTTP server with sebuf handlers
- **NATS publisher**: `internal/nats/publisher.go` - publishes to JetStream
- **Warehouse sink**: `internal/warehouse/` - consumes from NATS, writes Parquet to S3

### 2. Infrastructure (docker-compose.yml) ✅
- NATS with JetStream
- MinIO (S3-compatible storage)
- PostgreSQL (for Hive metastore + Redash)
- Hive Metastore
- Trino 435 (pinned for stability)
- Redash (visualization)
- Redis (for Redash)
- Causality server and warehouse-sink

### 3. Makefile Commands ✅
- `make dev` - Clean start everything, wait for health, create Trino tables
- `make docker-rebuild` - Clean rebuild
- `make test-event` / `make test-batch` / `make test-load` - Send test events
- `make trino-init` - Create schema/tables
- `make trino-cli` - Open Trino CLI

## BLOCKED: S3/Trino Integration

### The Problem
Trino 435 can't create external tables with `s3a://` URLs because:
1. Hive Metastore validates s3a:// paths using hadoop-aws library ✅
2. Trino 435 does NOT bundle hadoop-aws library ❌
3. Error: `java.lang.ClassNotFoundException: Class org.apache.hadoop.fs.s3a.S3AFileSystem not found`

### The Solution
**Switch from Hive tables to Iceberg tables.**

Read `S3_TRINO_FIX_PLAN.md` for the full solution.

## What to Tell Claude After Context Clear

```
Resume Causality project. Read S3_TRINO_FIX_PLAN.md for the solution.

Current issue: Trino can't create external tables with s3a:// URLs because Hive Metastore
validates paths using hadoop-aws, but Trino 435 doesn't bundle hadoop-aws.

Solution: Switch from Hive tables to Iceberg tables. Iceberg has native S3 support.

Tasks:
1. Create docker/trino/catalog/iceberg.properties with Iceberg config
2. Update Makefile trino-init to create Iceberg schema/tables (use s3:// URLs)
3. Test with: make dev
4. Then test event flow: make test-load && make trino-sync && make trino-stats
5. After S3/Trino works, build the analysis engine for anomaly detection
```

## Key Files
- `S3_TRINO_FIX_PLAN.md` - **READ THIS FIRST** - Full solution details
- `docker-compose.yml` - All infrastructure
- `Makefile` - All commands
- `docker/trino/catalog/hive.properties` - Current (broken) Hive config
- `internal/warehouse/parquet.go` - Parquet schema (EventRow)
