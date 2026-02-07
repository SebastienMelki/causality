# Technology Stack

**Analysis Date:** 2026-02-05

## Languages

**Primary:**
- Go 1.24.7 - All backend services (HTTP server, warehouse sink, reaction engine), protocol buffer generation

**Secondary:**
- SQL - PostgreSQL database schema and queries via `github.com/lib/pq`
- Protocol Buffers (proto3) - Event definitions and API contracts via `google.golang.org/protobuf`

## Runtime

**Environment:**
- Go 1.24+ required (toolchain 1.24.12 specified in `go.mod`)
- Alpine Linux 3.19 for production container images

**Package Manager:**
- Go modules - Package management via `go.mod` and `go.sum`
- Lockfile: `go.sum` present

## Frameworks

**Core:**
- Standard Go `net/http` library - HTTP server implementation in `internal/gateway`
- NATS JetStream (`github.com/nats-io/nats.go`) v1.39.1 - Event streaming and pub/sub
- Protocol Buffers (`google.golang.org/protobuf`) v1.36.11 - Serialization and API definitions
- buf (`buf.build/gen/go/bufbuild/protovalidate`) - Protocol buffer validation and generation

**Storage & Processing:**
- Parquet (`github.com/parquet-go/parquet-go`) v0.25.0 - Columnar data format for warehouse sink
- AWS SDK v2 (`github.com/aws/aws-sdk-go-v2`) - S3-compatible object storage operations
- PostgreSQL (`github.com/lib/pq`) v1.10.9 - Reaction engine rule/webhook/anomaly configuration storage

**Configuration & Utilities:**
- `github.com/caarlos0/env/v10` v10.0.0 - Environment variable parsing
- `github.com/google/uuid` v1.6.0 - UUID generation for object keys
- `golang.org/x/time` v0.9.0 - Rate limiting support
- `github.com/SebastienMelki/sebuf` v0.2.0 - Custom protobuf code generation for HTTP handlers

**Testing:**
- Built-in Go testing framework (`testing` package)
- Test files present in source tree with `_test.go` naming convention

## Key Dependencies

**Critical:**
- `github.com/nats-io/nats.go` v1.39.1 - Event streaming backbone; used for publishing events and consuming by warehouse sink and reaction engine
- `github.com/parquet-go/parquet-go` v0.25.0 - Storage format for data warehouse; enables efficient columnar storage in MinIO
- `github.com/aws/aws-sdk-go-v2` (core and s3) - S3 client for MinIO integration; enables Parquet file uploads
- `github.com/lib/pq` v1.10.9 - PostgreSQL connection driver for reaction engine rule engine and webhook configuration

**Infrastructure:**
- `google.golang.org/protobuf` v1.36.11 - Protocol buffer code generation and serialization
- `buf.build/gen/go/bufbuild/protovalidate` - Proto validation framework
- `github.com/SebastienMelki/sebuf` v0.2.0 - HTTP handler and OpenAPI generation from protos

**Transitive Dependencies:**
- AWS SDK internals: ec2/imds, credentials, endpoints, s3shared, sso, ssooidc, sts, smithy-go
- Compression libraries: brotli, lz4, zstd via transitive dependencies
- CEL expression language (`cel.dev/expr`, `google.golang.org/genproto`) - Used for rule evaluation in reaction engine
- ANTLR4 for expression parsing

## Configuration

**Environment:**
- Environment variables parsed using `github.com/caarlos0/env/v10`
- No `.env` file dependency; all config via env vars with defaults
- Supports environment variable prefixes for grouped config (e.g., `S3_`, `NATS_`, `HTTP_`, `BATCH_`, `PARQUET_`, etc.)

**Build:**
- Multi-stage Dockerfile (`Dockerfile`) builds three separate service images: server, warehouse-sink, reaction-engine
- Uses CGO_ENABLED=0 for static binaries
- Build optimization: `-ldflags="-w -s"` strips symbols for smaller binary size

**Configuration Files:**
- `go.mod` - Go module definition
- `go.sum` - Dependency checksums
- `buf.yaml` - Protocol buffer module and code generation configuration
- `docker-compose.yml` - Local development environment definition
- `Makefile` - Build and development commands

## Platform Requirements

**Development:**
- Go 1.24+ toolchain
- Docker and Docker Compose for local development environment
- buf CLI for protocol buffer generation (`make install-tools` installs it)
- golangci-lint for linting
- Standard Unix tools (curl, wget, git)

**Production:**
- **Deployment Target**: Docker containers running on Linux (Alpine Linux 3.19)
- **Service Coordinator**: Docker Compose for orchestration (development) or Kubernetes/similar for production
- **External Services Required**:
  - NATS 2.10+ server with JetStream enabled
  - PostgreSQL 16+ (for Hive Metastore and Reaction Engine config)
  - MinIO or AWS S3 (object storage)
  - Hive Metastore 4.0.0+ (schema registry)
  - Trino 444 (SQL query engine for analytics)
  - Redis 7+ (required for Redash)
  - Redash 10.1.0+ (optional, for visualization)

**Network:**
- Services communicate over Docker network `causality-network` in local development
- All services expect container-to-container DNS resolution

## Build Artifacts

**Docker Images (Multi-stage):**
- `server` - HTTP event ingestion server, based on `alpine:3.19`
- `warehouse-sink` - NATS consumer to Parquet/S3 writer, based on `alpine:3.19`
- `reaction-engine` - Rule evaluation and anomaly detection, based on `alpine:3.19`

All images:
- Run as non-root user (`appuser`)
- Include CA certificates for TLS
- Built from same `Dockerfile` with different targets

**Binaries:**
- `causality-server` - HTTP server (port 8080)
- `warehouse-sink` - Warehouse consumer
- `reaction-engine` - Rule engine (requires PostgreSQL)

---

*Stack analysis: 2026-02-05*
