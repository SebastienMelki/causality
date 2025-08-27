# Causality Architecture

## Overview

Causality detects application modifications by analyzing event patterns from mobile and web applications.

## System Components

```
Mobile/Web Apps → SDKs → HTTP Server → Analysis Engine → Anomaly Detection
```

### 1. Client SDKs

- **Mobile SDK** (gomobile): iOS and Android native bindings
- **Web SDK** (WASM): Browser-compatible JavaScript module

### 2. HTTP Server

Central event collection point handling:
- RESTful API endpoints
- Protocol Buffer deserialization
- Event validation
- Request/response management
- TLS encryption

### 3. Analysis Engine

Processes events to detect behavioral anomalies:
- Pattern matching
- Statistical analysis
- Real-time processing

## Data Flow

1. **Event Generation**: App creates custom events
2. **SDK Processing**: Events serialized with Protocol Buffers
3. **Transmission**: Sent to HTTP server via RESTful API over HTTPS
4. **Analysis**: Behavioral patterns analyzed for anomalies
5. **Detection**: Modifications identified and reported

## Project Structure

```
causality/
├── cmd/server/       # HTTP server
├── internal/         # Core logic
│   ├── analysis/     # Anomaly detection
│   ├── events/       # Event processing
│   └── http/         # Server implementation
├── mobile/           # Mobile SDK source
├── wasm/             # WebAssembly SDK
└── proto/            # Protocol Buffer definitions
```

## Security

- HTTPS encryption for all API requests
- Event validation and sanitization
- Rate limiting per client
- Authentication via API keys