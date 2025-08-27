# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Causality is a behavioral analysis system that detects application modifications by analyzing event patterns from mobile (iOS/Android) and web applications. It uses:
- **gomobile** for generating mobile SDKs
- **WebAssembly** for web SDK
- **Protocol Buffers** for event definitions
- **HTTP server** for event collection
- **Behavioral analysis** for anomaly detection

Module: github.com/SebastienMelki/causality (Go 1.25.0)

## Common Commands

### Core Development
- **Initialize dependencies**: `go mod tidy`
- **Build server**: `go build -o bin/causality-server ./cmd/server`
- **Run server**: `./bin/causality-server --port 8080`
- **Run all tests**: `go test ./...`
- **Run tests with coverage**: `go test -cover ./...`
- **Run specific test**: `go test -run TestName ./...`
- **Run benchmarks**: `go test -bench=. ./...`

### Mobile SDK (gomobile)
- **Install gomobile**: `go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`
- **Build Android SDK**: `gomobile bind -target=android -o mobile/causality.aar ./mobile`
- **Build iOS SDK**: `gomobile bind -target=ios -o mobile/Causality.xcframework ./mobile`
- **Build both platforms**: `gomobile bind -target=ios,android ./mobile`

### WebAssembly SDK
- **Build WASM**: `GOOS=js GOARCH=wasm go build -o wasm/causality.wasm ./wasm`
- **Copy WASM support**: `cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" wasm/`
- **Test in browser**: `goexec 'http.ListenAndServe(":8080", http.FileServer(http.Dir("wasm")))'`

### Protocol Buffers
- **Install protoc plugins**: 
  ```
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```
- **Compile proto files**: `protoc --go_out=. --go-grpc_out=. proto/*.proto`
- **Regenerate all protos**: `go generate ./...`

### Code Quality
- **Format code**: `go fmt ./...`
- **Run linter**: `go vet ./...`
- **Run staticcheck** (if installed): `staticcheck ./...`
- **Run golangci-lint** (if installed): `golangci-lint run`

### Module Management
- **Update dependencies**: `go get -u ./...`
- **Vendor dependencies**: `go mod vendor`
- **Verify dependencies**: `go mod verify`
- **Clean module cache**: `go clean -modcache`

## Project Structure

```
causality/
├── cmd/
│   ├── server/         # HTTP server for receiving events
│   └── cli/            # Command-line tools for testing and management
├── internal/
│   ├── analysis/       # Behavioral analysis and anomaly detection
│   ├── events/         # Event processing and validation
│   ├── storage/        # Event storage and retrieval
│   └── http/           # HTTP server implementation
├── pkg/
│   ├── sdk/            # Shared SDK code between mobile and WASM
│   └── proto/          # Generated protocol buffer code
├── mobile/             # gomobile SDK implementation
│   ├── client.go       # Mobile client implementation
│   └── events.go       # Mobile event handlers
├── wasm/               # WebAssembly SDK implementation
│   ├── main.go         # WASM entry point
│   └── client.go       # Web client implementation
├── proto/              # Protocol buffer definitions
│   └── events.proto    # Event message definitions
├── ui/                 # Web UI for event definition
└── examples/           # Example integrations
    ├── ios/            # iOS example app
    ├── android/        # Android example app
    └── web/            # Web example app
```

## Architecture Overview

### Data Flow
1. **Event Generation**: Mobile/Web apps generate custom events
2. **SDK Processing**: Events are serialized using Protocol Buffers
3. **Network Transport**: Events sent to HTTP server via RESTful API
4. **Server Reception**: HTTP server validates and stores events
5. **Analysis**: Behavioral analysis engine processes event patterns
6. **Detection**: Anomalies indicate potential app modifications

### Key Components

#### HTTP Server
- Handles RESTful API requests
- Protocol Buffer deserialization
- Event validation and storage
- Event forwarding to analysis engine

#### Mobile SDK (gomobile)
- Provides native bindings for iOS/Android
- Event batching for network efficiency
- Automatic reconnection handling
- Offline event queueing

#### WebAssembly SDK
- Browser-compatible event tracking
- WebSocket connection to server
- LocalStorage for offline events
- Minimal bundle size

#### Analysis Engine
- Statistical modeling of normal behavior
- Real-time anomaly detection
- Pattern matching for known modifications
- Alert generation for suspicious activity

## Development Guidelines

### Code Organization
- Use `internal/` for non-exported application logic
- Keep protocol definitions in `proto/` directory
- Place generated code in `pkg/proto/`
- Maintain separate `mobile/` and `wasm/` for platform-specific code
- Write comprehensive tests alongside implementation

### Testing Strategy
- Unit tests for all business logic
- Integration tests for HTTP API endpoints
- End-to-end tests with example apps
- Behavioral analysis validation with test datasets
- Performance benchmarks for event processing

### Common Patterns
- Use context.Context for cancellation
- Implement graceful shutdown for servers
- Use sync.Pool for Protocol Buffer marshaling
- Apply circuit breaker pattern for network calls
- Implement exponential backoff for retries