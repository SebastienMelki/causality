.PHONY: help build clean test lint install generate mobile wasm

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build HTTP server binary
	@echo "Building HTTP server..."
	@mkdir -p bin
	@go build -o bin/causality-server ./cmd/server

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/ coverage/

test: ## Run tests
	@echo "Running tests..."
	@go test ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...

install: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod tidy
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install golang.org/x/mobile/cmd/gomobile@latest
	@gomobile init

generate: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go-grpc_out=. proto/*.proto

mobile: ## Build mobile SDKs
	@echo "Building mobile SDKs..."
	@gomobile bind -target=android -o mobile/causality.aar ./mobile
	@gomobile bind -target=ios -o mobile/Causality.xcframework ./mobile

wasm: ## Build WebAssembly SDK
	@echo "Building WASM..."
	@GOOS=js GOARCH=wasm go build -o wasm/causality.wasm ./wasm

.DEFAULT_GOAL := help