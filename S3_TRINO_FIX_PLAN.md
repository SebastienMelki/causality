# S3/Trino Integration Fix Plan

## The Problem

We have a compatibility issue between Trino and Hive Metastore for S3 access:

1. **Hive Metastore** uses Hadoop's `s3a://` filesystem (requires hadoop-aws library)
2. **Trino 435** has native `s3://` support but does NOT bundle hadoop-aws
3. **Trino 479 (latest)** has new native S3 config but was stuck in SERVER_STARTING_UP for 5+ minutes

When creating external tables with `s3a://` locations, Hive Metastore validates the path using its own filesystem - which works. But Trino can't read from `s3a://` without hadoop-aws.

## Solution: Use Iceberg Instead of Hive Tables

**Why Iceberg:**
- Iceberg connector has built-in native S3 support (no hadoop-aws needed)
- Uses `s3://` URLs which Trino handles natively
- No Hive Metastore S3 validation issues (metadata stored differently)
- Better table format anyway (ACID, time travel, schema evolution)
- Works with same MinIO setup

**Changes Required:**

### 1. Add Iceberg catalog to Trino (`docker/trino/catalog/iceberg.properties`)
```properties
connector.name=iceberg
iceberg.catalog.type=hive_metastore
hive.metastore.uri=thrift://hive-metastore:9083
hive.s3.endpoint=http://minio:9000
hive.s3.path-style-access=true
hive.s3.aws-access-key=minioadmin
hive.s3.aws-secret-key=minioadmin
hive.s3.ssl.enabled=false
iceberg.file-format=PARQUET
```

### 2. Update Makefile trino-init to use Iceberg
```makefile
trino-init:
	@docker exec causality-trino trino --execute "CREATE SCHEMA IF NOT EXISTS iceberg.causality WITH (location = 's3://causality-events/')"
	@docker exec causality-trino trino --execute "CREATE TABLE IF NOT EXISTS iceberg.causality.events (...) WITH (format = 'PARQUET', partitioning = ARRAY['app_id', 'year', 'month', 'day', 'hour'])"
```

### 3. Update warehouse sink to write Iceberg format
- Change parquet writer to use Iceberg table format
- Or: Write raw parquet and use Iceberg's `register_table` procedure

### 4. Keep Trino 435 (stable) or try 479 with longer timeout

## Alternative: Add hadoop-aws to Trino (More Complex)

If you must use Hive tables with s3a://:
1. Download hadoop-aws-3.3.4.jar and aws-java-sdk-bundle-1.12.x.jar
2. Mount them into Trino container at `/usr/lib/trino/plugin/hive/`
3. This adds ~200MB and complexity

## Files to Modify

1. `docker/trino/catalog/iceberg.properties` - NEW
2. `Makefile` - Update trino-init commands
3. `internal/warehouse/parquet.go` - May need updates for Iceberg format
4. `docker-compose.yml` - Keep as-is

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

## Current State

- Docker compose works
- All services start (NATS, MinIO, Postgres, Hive Metastore, Redash, Trino)
- HTTP server receives events
- Warehouse sink writes Parquet to MinIO
- Trino 435 starts and connects to Hive Metastore
- BLOCKED: Cannot create external tables pointing to S3
