.PHONY: help build clean test lint lint-fix install generate mobile wasm \
        install-tools install-sebuf buf-generate buf-lint \
        build-server build-sink docker-up docker-down docker-build \
        test-unit test-e2e test-coverage

# Default target
.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================
help: ## Show this help message
	@echo "Causality Event Processing System"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# =============================================================================
# Core Development
# =============================================================================
build: build-server build-sink build-reaction ## Build all binaries

build-server: ## Build HTTP server binary
	@echo "Building HTTP server..."
	@mkdir -p bin
	@go build -o bin/causality-server ./cmd/server

build-sink: ## Build warehouse sink binary
	@echo "Building warehouse sink..."
	@mkdir -p bin
	@go build -o bin/warehouse-sink ./cmd/warehouse-sink

build-reaction: ## Build reaction engine binary
	@echo "Building reaction engine..."
	@mkdir -p bin
	@go build -o bin/reaction-engine ./cmd/reaction-engine

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/ coverage/ api/openapi/

run-server: build-server ## Run HTTP server locally
	@echo "Running HTTP server..."
	@./bin/causality-server

run-sink: build-sink ## Run warehouse sink locally
	@echo "Running warehouse sink..."
	@./bin/warehouse-sink

run-reaction: build-reaction ## Run reaction engine locally
	@echo "Running reaction engine..."
	@./bin/reaction-engine

# =============================================================================
# Testing
# =============================================================================
test: test-unit ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	@go test -coverprofile=coverage/coverage.out ./...
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report: coverage/coverage.html"

test-e2e: ## Run end-to-end tests (requires docker-compose)
	@echo "Running E2E tests..."
	@go test -v -tags=e2e ./tests/e2e/...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# =============================================================================
# Code Quality
# =============================================================================
lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	@golangci-lint run --fix ./...

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

check: fmt vet lint ## Run all code quality checks

# =============================================================================
# Dependencies & Tools
# =============================================================================
install: install-tools ## Install all dependencies and tools
	@echo "Installing Go dependencies..."
	@go mod tidy

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

install-sebuf: ## Install sebuf plugins for HTTP handler generation
	@echo "Installing sebuf plugins..."
	@go install github.com/SebastienMelki/sebuf/cmd/protoc-gen-go-http@latest
	@go install github.com/SebastienMelki/sebuf/cmd/protoc-gen-openapiv3@latest

# =============================================================================
# Protocol Buffers
# =============================================================================
buf-generate: ## Generate protobuf code with buf
	@echo "Generating protobuf code..."
	@buf generate

buf-lint: ## Lint proto files
	@echo "Linting proto files..."
	@buf lint

buf-breaking: ## Check for breaking changes
	@echo "Checking for breaking changes..."
	@buf breaking --against '.git#branch=main'

generate: buf-generate ## Generate all code (alias for buf-generate)

# =============================================================================
# Docker
# =============================================================================
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@docker-compose build

docker-up: ## Start local development environment
	@echo "Starting local environment..."
	@docker-compose up -d
	@echo ""
	@echo "Services started:"
	@echo "  - HTTP Server:    http://localhost:8080"
	@echo "  - NATS:           localhost:4222 (monitoring: http://localhost:8222)"
	@echo "  - MinIO:          http://localhost:9001 (minioadmin/minioadmin)"
	@echo "  - Trino:          http://localhost:8085"
	@echo "  - Redash:         http://localhost:5050"
	@echo "  - Hive Metastore: localhost:9083"
	@echo "  - PostgreSQL:     localhost:5432"

docker-down: ## Stop local development environment
	@echo "Stopping local environment..."
	@docker-compose down

docker-restart: ## Restart all services
	@docker-compose down
	@docker-compose up -d

docker-logs: ## View logs from all services
	@docker-compose logs -f

docker-logs-server: ## View server logs
	@docker-compose logs -f causality-server

docker-logs-sink: ## View warehouse sink logs
	@docker-compose logs -f warehouse-sink

docker-logs-trino: ## View Trino logs
	@docker-compose logs -f trino

docker-logs-reaction: ## View reaction engine logs
	@docker-compose logs -f reaction-engine

docker-clean: ## Remove all containers and volumes
	@echo "Cleaning Docker resources..."
	@docker-compose down -v --remove-orphans

docker-rebuild: docker-clean docker-build docker-up ## Clean rebuild everything

docker-ps: ## Show running containers
	@docker-compose ps

# =============================================================================
# Trino & Data
# =============================================================================
trino-cli: ## Open Trino CLI
	@docker exec -it causality-trino trino

trino-init: ## Create Trino schema and tables
	@echo "Creating Trino schema and tables..."
	@docker exec causality-trino trino --execute "CREATE SCHEMA IF NOT EXISTS hive.causality WITH (location = 's3a://causality-events/')"
	@docker exec causality-trino trino --execute "CREATE TABLE IF NOT EXISTS hive.causality.events (id VARCHAR, device_id VARCHAR, timestamp_ms BIGINT, correlation_id VARCHAR, event_category VARCHAR, event_type VARCHAR, platform VARCHAR, os_version VARCHAR, app_version VARCHAR, build_number VARCHAR, device_model VARCHAR, manufacturer VARCHAR, screen_width INTEGER, screen_height INTEGER, locale VARCHAR, timezone VARCHAR, network_type VARCHAR, carrier VARCHAR, is_jailbroken BOOLEAN, is_emulator BOOLEAN, sdk_version VARCHAR, payload_json VARCHAR, app_id VARCHAR, year INTEGER, month INTEGER, day INTEGER, hour INTEGER) WITH (format = 'PARQUET', partitioned_by = ARRAY['app_id', 'year', 'month', 'day', 'hour'], external_location = 's3a://causality-events/events/')"
	@echo "Tables created successfully"

trino-sync: ## Sync Trino partitions from S3
	@echo "Syncing partitions..."
	@docker exec causality-trino trino --execute "CALL hive.system.sync_partition_metadata('causality', 'events', 'FULL')"

trino-query: ## Query recent events (usage: make trino-query SQL="SELECT * FROM hive.causality.events LIMIT 10")
	@docker exec causality-trino trino --execute "$(SQL)"

trino-count: ## Count events in warehouse
	@docker exec causality-trino trino --execute "SELECT count(*) as total_events FROM hive.causality.events"

trino-stats: ## Show event statistics
	@docker exec causality-trino trino --execute "SELECT event_category, event_type, count(*) as count FROM hive.causality.events GROUP BY event_category, event_type ORDER BY count DESC"

# =============================================================================
# Testing & Events
# =============================================================================
test-event: ## Send a single test event
	@curl -s -X POST http://localhost:8080/v1/events/ingest \
		-H "Content-Type: application/json" \
		-d '{"event":{"appId":"test-app","deviceId":"device-001","screenView":{"screenName":"HomeScreen"}}}' | jq .

test-batch: ## Send a batch of test events
	@curl -s -X POST http://localhost:8080/v1/events/batch \
		-H "Content-Type: application/json" \
		-d '{"events":[{"appId":"test-app","deviceId":"d1","screenView":{"screenName":"Home"}},{"appId":"test-app","deviceId":"d2","buttonTap":{"buttonId":"btn-login","screenName":"Home"}},{"appId":"test-app","deviceId":"d3","userLogin":{"userId":"user-123","method":"email"}}]}' | jq .

test-load: ## Send 100 test events for load testing
	@echo "Sending 100 test events..."
	@for i in $$(seq 1 10); do \
		curl -s -X POST http://localhost:8080/v1/events/batch \
			-H "Content-Type: application/json" \
			-d '{"events":[{"appId":"load-test","deviceId":"d'$$i'","screenView":{"screenName":"Screen'$$i'"}},{"appId":"load-test","deviceId":"d'$$i'","buttonTap":{"buttonId":"btn-'$$i'","screenName":"Screen'$$i'"}},{"appId":"load-test","deviceId":"d'$$i'","userLogin":{"userId":"user-'$$i'","method":"email"}},{"appId":"load-test","deviceId":"d'$$i'","productView":{"productId":"prod-'$$i'","productName":"Product '$$i'","priceCents":999}},{"appId":"load-test","deviceId":"d'$$i'","addToCart":{"productId":"prod-'$$i'","quantity":1,"priceCents":999}},{"appId":"load-test","deviceId":"d'$$i'","purchaseComplete":{"orderId":"order-'$$i'","totalCents":999,"currency":"USD"}},{"appId":"load-test","deviceId":"d'$$i'","appStart":{"isColdStart":true,"launchDurationMs":150}},{"appId":"load-test","deviceId":"d'$$i'","customEvent":{"eventName":"test_event_'$$i'","stringParams":{"key":"value"}}},{"appId":"load-test","deviceId":"d'$$i'","screenExit":{"screenName":"Screen'$$i'","durationMs":5000}},{"appId":"load-test","deviceId":"d'$$i'","networkChange":{}}]}' > /dev/null; \
	done
	@echo "Done! Sent 100 events"

test-random: ## Send random events with variation (for realistic graphs)
	@echo "Sending random events..."
	@for round in $$(seq 1 20); do \
		device=$$((RANDOM % 100)); \
		screen_count=$$((RANDOM % 10 + 1)); \
		for s in $$(seq 1 $$screen_count); do \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","screenView":{"screenName":"Screen'$$((RANDOM % 20))'"}}}' > /dev/null; \
		done; \
		btn_count=$$((RANDOM % 8)); \
		for b in $$(seq 1 $$btn_count); do \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","buttonTap":{"buttonId":"btn-'$$((RANDOM % 15))'","screenName":"Screen'$$((RANDOM % 20))'"}}}' > /dev/null; \
		done; \
		if [ $$((RANDOM % 3)) -eq 0 ]; then \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","userLogin":{"userId":"user-'$$((RANDOM % 50))'","method":"email"}}}' > /dev/null; \
		fi; \
		if [ $$((RANDOM % 4)) -eq 0 ]; then \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","productView":{"productId":"prod-'$$((RANDOM % 30))'","productName":"Product","priceCents":'$$((RANDOM % 10000 + 100))'}}}' > /dev/null; \
		fi; \
		if [ $$((RANDOM % 6)) -eq 0 ]; then \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","addToCart":{"productId":"prod-'$$((RANDOM % 30))'","quantity":'$$((RANDOM % 5 + 1))',"priceCents":'$$((RANDOM % 10000 + 100))'}}}' > /dev/null; \
		fi; \
		if [ $$((RANDOM % 10)) -eq 0 ]; then \
			curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
				-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","purchaseComplete":{"orderId":"order-'$$RANDOM'","totalCents":'$$((RANDOM % 50000 + 500))',"currency":"USD"}}}' > /dev/null; \
		fi; \
		curl -s -X POST http://localhost:8080/v1/events/ingest -H "Content-Type: application/json" \
			-d '{"event":{"appId":"app-'$$((RANDOM % 3))'","deviceId":"device-'$$device'","appStart":{"isColdStart":'$$([ $$((RANDOM % 2)) -eq 0 ] && echo true || echo false)',"launchDurationMs":'$$((RANDOM % 2000 + 100))'}}}' > /dev/null; \
		echo -n "."; \
	done
	@echo ""
	@echo "Done! Sent random events with variation"

health: ## Check server health
	@curl -s http://localhost:8080/health | jq .

ready: ## Check server readiness
	@curl -s http://localhost:8080/ready | jq .

# =============================================================================
# NATS
# =============================================================================
nats-streams: ## List NATS streams
	@curl -s "http://localhost:8222/jsz?streams=true" | jq '.streams'

nats-consumers: ## List NATS consumers
	@curl -s "http://localhost:8222/jsz?consumers=true" | jq '.streams[].consumers'

nats-info: ## Show NATS server info
	@curl -s "http://localhost:8222/varz" | jq '{server_id, version, max_connections, subscriptions, messages_in: .in_msgs, messages_out: .out_msgs}'

# =============================================================================
# MinIO
# =============================================================================
minio-ls: ## List objects in MinIO bucket
	@docker exec causality-minio mc ls local/causality-events --recursive

minio-size: ## Show bucket size
	@docker exec causality-minio mc du local/causality-events

# =============================================================================
# Mobile SDK (gomobile)
# =============================================================================
.PHONY: mobile-setup mobile-ios mobile-android mobile-all mobile-clean mobile-test

mobile-setup: ## Install gomobile and initialize
	@echo "Installing gomobile..."
	@go install golang.org/x/mobile/cmd/gomobile@latest
	@gomobile init

mobile-ios: ## Build iOS SDK (.xcframework)
	@./scripts/gomobile-build.sh ios

mobile-android: ## Build Android SDK (.aar)
	@./scripts/gomobile-build.sh android

mobile-all: ## Build both iOS and Android SDKs
	@./scripts/gomobile-build.sh all

mobile-clean: ## Clean mobile build artifacts
	@echo "Cleaning mobile build artifacts..."
	@rm -rf build/mobile

mobile-test: ## Run mobile SDK tests and verify no CGO
	@echo "Running mobile SDK tests..."
	@go test ./sdk/mobile/... -v -race
	@echo "Verifying no CGO dependencies..."
	@go list -f '{{.CgoFiles}}' ./sdk/mobile/... | grep -v '^\[\]$$' && echo "ERROR: CGO files found!" && exit 1 || echo "OK: No CGO dependencies"

# =============================================================================
# WebAssembly SDK
# =============================================================================
wasm: ## Build WebAssembly SDK
	@echo "Building WASM..."
	@mkdir -p wasm
	@GOOS=js GOARCH=wasm go build -o wasm/causality.wasm ./wasm
	@cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" wasm/

# =============================================================================
# Utilities
# =============================================================================
verify: ## Verify module dependencies
	@echo "Verifying dependencies..."
	@go mod verify

tidy: ## Tidy and verify module dependencies
	@echo "Tidying dependencies..."
	@go mod tidy
	@go mod verify

update-deps: ## Update all dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# =============================================================================
# Quick Start
# =============================================================================
quickstart: install docker-up ## Quick start: install deps and start environment
	@echo ""
	@echo "Quick start complete!"
	@echo ""
	@echo "Wait for services to be ready, then run:"
	@echo "  make health        - Check server health"
	@echo "  make test-event    - Send a test event"
	@echo "  make test-load     - Send 100 test events"
	@echo "  make trino-init    - Create Trino tables"
	@echo "  make trino-sync    - Sync partitions"
	@echo "  make trino-stats   - View event statistics"

dev: docker-clean docker-up dev-wait trino-init ## Start dev environment with tables (clean start)
	@echo "Dev environment ready!"
	@echo "  make test-event  - Send test event"
	@echo "  make trino-cli   - Open Trino CLI"

dev-wait: ## Wait for services to be healthy
	@echo "Waiting for services to be ready..."
	@until curl -s http://localhost:8080/health > /dev/null 2>&1; do sleep 2; done
	@echo "  ✓ HTTP Server ready"
	@echo "Waiting for Trino to fully initialize..."
	@until docker exec causality-trino trino --execute "SELECT 1" > /dev/null 2>&1; do sleep 3; done
	@echo "  ✓ Trino ready"
	@echo "All services ready!"

dev-logs: docker-logs ## Tail all logs (alias)

dev-rebuild: ## Rebuild and restart only the Go services (uses cache)
	@docker-compose up -d --build --force-recreate causality-server warehouse-sink reaction-engine

dev-rebuild-clean: ## Rebuild Go services from scratch (no cache)
	@echo "Rebuilding Go services without cache..."
	@docker-compose build --no-cache causality-server warehouse-sink reaction-engine
	@docker-compose up -d --force-recreate causality-server warehouse-sink reaction-engine
	@echo "Waiting for services..."
	@sleep 5
	@echo "Services rebuilt and restarted"

api-docs: ## Open OpenAPI spec
	@echo "OpenAPI spec: api/openapi/EventService.openapi.yaml"
	@cat api/openapi/EventService.openapi.yaml

.PHONY: all
all: install generate build test ## Build everything
