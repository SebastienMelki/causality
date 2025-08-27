# Causality

A behavioral analysis system that detects application modifications by analyzing event patterns and identifying anomalies from expected behavior.

## Overview

Causality generates cross-platform SDKs (mobile via gomobile, web via WASM) that enable applications to send custom events to a central HTTP server. By analyzing these events, the system can detect when an application has been modified or tampered with based on deviations from normal behavioral patterns.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Mobile Apps   │     │    Web Apps     │     │   Event UI      │
│  (iOS/Android)  │     │   (Browser)     │     │  (Definition)   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                         │
    ┌────▼────┐             ┌────▼────┐                   │
    │ Mobile  │             │  WASM   │                   │
    │   SDK   │             │   SDK   │                   │
    └────┬────┘             └────┬────┘                   │
         │                       │                         │
         └───────────┬───────────┘                         │
                     │                                     │
              ┌──────▼──────┐                      ┌──────▼──────┐
              │ HTTP Server  │◄─────────────────────│   Protobuf  │
              │   (Events)   │                      │ Definitions │
              └──────┬───────┘                      └─────────────┘
                     │
           ┌─────────▼─────────┐
           │ Analysis Engine   │
           │ (Anomaly Detection)│
           └───────────────────┘
```

### Components

- **Mobile SDK**: Generated using gomobile for iOS and Android integration
- **Web SDK**: Compiled to WebAssembly for browser-based applications
- **HTTP Server**: RESTful API for receiving and processing events from all clients
- **Protocol Buffers**: Define the structure of custom events
- **Event UI**: Interface for defining and managing custom event schemas
- **Analysis Engine**: Detects behavioral anomalies indicating app modifications

## Features

- Cross-platform event tracking (iOS, Android, Web)
- Custom event definitions via Protocol Buffers
- RESTful API for event collection
- Behavioral pattern analysis
- Anomaly detection for tampered applications
- Visual event definition interface

## Getting Started

### Prerequisites

- Go 1.25.0 or later
- Protocol Buffer compiler (protoc)
- gomobile for mobile SDK generation
- Node.js (for UI components)

### Installation

```bash
# Clone the repository
git clone https://github.com/SebastienMelki/causality
cd causality

# Install Go dependencies
go mod tidy

# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Project Structure

```
causality/
├── cmd/
│   ├── server/        # HTTP server application
│   └── cli/           # Command-line tools
├── internal/
│   ├── analysis/      # Behavioral analysis engine
│   ├── events/        # Event processing logic
│   └── storage/       # Event storage layer
├── pkg/
│   ├── sdk/           # Shared SDK code
│   └── proto/         # Protocol buffer definitions
├── mobile/            # Mobile SDK source
├── wasm/              # WebAssembly SDK source
├── ui/                # Event definition UI
└── proto/             # Protocol buffer schemas
```

## Development

### Building the Server

```bash
go build -o bin/causality-server ./cmd/server
```

### Generating Mobile SDK

```bash
# For Android
gomobile bind -target=android -o mobile/causality.aar ./mobile

# For iOS
gomobile bind -target=ios -o mobile/Causality.xcframework ./mobile
```

### Building WASM SDK

```bash
GOOS=js GOARCH=wasm go build -o wasm/causality.wasm ./wasm
```

### Compiling Protocol Buffers

```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```

### Running Tests

```bash
go test ./...
```

## Usage

### Server

```bash
# Start the HTTP server
./bin/causality-server --port 8080
```

### Mobile Integration

```swift
// iOS Example
import Causality

let client = CausalityClient(serverAddress: "https://server:8080")
client.sendEvent(CustomEvent(type: "user_action", data: eventData))
```

### Web Integration

```javascript
// Web Example
import { CausalityClient } from './causality.js';

const client = new CausalityClient('https://server:8080');
await client.sendEvent({
  type: 'user_action',
  data: eventData
});
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.