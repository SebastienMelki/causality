# Causality Architecture

## Overview

Causality is an event analytics platform that collects behavioral events from mobile and web applications, stores them in a data warehouse, and enables SQL-based analytics.

## System Architecture

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
              ┌──────▼──────┐
              │  Warehouse  │ Parquet files
              │    Sink     │ → MinIO (S3)
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │    Trino    │ SQL Analytics
              └──────┬──────┘
                     │
              ┌──────▼──────┐
              │   Redash    │ Visualization
              └─────────────┘
```

## Components

### 1. HTTP Server (`cmd/server`)

Central event collection point:
- RESTful API endpoints for event ingestion
- Protocol Buffer request/response handling
- Event validation and enrichment
- Publishes events to NATS JetStream

**Endpoints:**
- `POST /v1/events/ingest` - Single event ingestion
- `POST /v1/events/batch` - Batch event ingestion
- `GET /health` - Health check
- `GET /ready` - Readiness check

**Configuration:**
- `HTTP_ADDR`: Listen address (default: `:8080`)
- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)

### 2. NATS JetStream

Event streaming and reliable delivery:
- Durable message storage
- At-least-once delivery guarantees
- Consumer groups for horizontal scaling
- Stream: `EVENTS`

### 3. Warehouse Sink (`cmd/warehouse-sink`)

Processes events from NATS and writes to storage:
- Consumes events from JetStream
- Batches events for efficient writes
- Converts to Apache Parquet format
- Uploads to MinIO (S3-compatible)
- Hive-style partitioning: `app_id/year/month/day/hour`

**Configuration:**
- `NATS_URL`: NATS server URL
- `S3_ENDPOINT`: MinIO/S3 endpoint
- `S3_BUCKET`: Bucket name (default: `causality-events`)
- `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY`: Credentials
- `BATCH_MAX_EVENTS`: Events per Parquet file (default: `1000`)
- `BATCH_FLUSH_INTERVAL`: Max time before flush (default: `30s`)

### 4. MinIO

S3-compatible object storage:
- Stores Parquet files
- Bucket: `causality-events`
- Path pattern: `events/app_id=X/year=Y/month=M/day=D/hour=H/*.parquet`

### 5. Hive Metastore

Schema registry for Trino:
- Stores table definitions
- Tracks partition metadata
- Uses PostgreSQL as backing store
- Configured with S3 (hadoop-aws) for path validation

### 6. Trino

SQL query engine:
- Queries Parquet files directly from S3
- Hive connector with metastore integration
- Supports standard SQL syntax
- Configured for Presto protocol compatibility (for Redash)

**Key Configuration** (`docker/trino/etc/`):
```properties
# config.properties
protocol.v1.alternate-header-name=Presto  # pyhive compatibility

# catalog/hive.properties
connector.name=hive
hive.metastore.uri=thrift://hive-metastore:9083
hive.s3.endpoint=http://minio:9000
hive.s3.path-style-access=true
```

### 7. Redash

Data visualization and dashboards:
- Auto-configured Trino data source
- SQL query interface
- Dashboard builder
- Alert configuration

## Data Flow

1. **Ingestion**
   - Client app sends event via HTTP POST
   - Server validates and enriches event
   - Event published to NATS JetStream

2. **Processing**
   - Warehouse sink consumes from NATS
   - Events batched by size or time
   - Converted to Parquet format

3. **Storage**
   - Parquet file uploaded to MinIO
   - Partitioned by app_id/year/month/day/hour
   - Hive metastore notified of new partition

4. **Query**
   - User runs SQL via Trino or Redash
   - Trino reads partition metadata from Hive
   - Parquet files fetched from MinIO
   - Results returned to client

## Event Schema

Events are stored with the following columns:

| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR | Unique event ID |
| device_id | VARCHAR | Device identifier |
| app_id | VARCHAR | Application identifier |
| timestamp_ms | BIGINT | Event timestamp (ms) |
| event_category | VARCHAR | Event category |
| event_type | VARCHAR | Event type name |
| platform | VARCHAR | iOS/Android/Web |
| os_version | VARCHAR | OS version |
| app_version | VARCHAR | App version |
| payload_json | VARCHAR | Event-specific data |
| year | INTEGER | Partition: year |
| month | INTEGER | Partition: month |
| day | INTEGER | Partition: day |
| hour | INTEGER | Partition: hour |

## Partitioning Strategy

Events are partitioned for query performance:

```
s3://causality-events/events/
  app_id=my-app/
    year=2024/
      month=1/
        day=15/
          hour=10/
            events-001.parquet
            events-002.parquet
```

This enables:
- Partition pruning for time-range queries
- Efficient app-specific queries
- Parallel processing by partition

## Security Considerations

- Internal network isolation via Docker network
- MinIO credentials configured via environment
- Redash admin auto-provisioned (change in production)
- No external network access by default

## Scaling Considerations

- **HTTP Server**: Stateless, horizontally scalable
- **NATS**: Clusterable for HA
- **Warehouse Sink**: Can run multiple instances with consumer groups
- **MinIO**: Supports distributed mode
- **Trino**: Supports worker nodes for distributed queries
