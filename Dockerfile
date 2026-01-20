# Multi-stage Dockerfile for building causality services

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/causality-server ./cmd/server

# Build warehouse-sink
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/warehouse-sink ./cmd/warehouse-sink

# Build reaction-engine
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/reaction-engine ./cmd/reaction-engine


# Server image
FROM alpine:3.19 AS server

RUN apk add --no-cache ca-certificates wget

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

COPY --from=builder /bin/causality-server /usr/local/bin/causality-server

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/causality-server"]


# Warehouse sink image
FROM alpine:3.19 AS warehouse-sink

RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

COPY --from=builder /bin/warehouse-sink /usr/local/bin/warehouse-sink

ENTRYPOINT ["/usr/local/bin/warehouse-sink"]


# Reaction engine image
FROM alpine:3.19 AS reaction-engine

RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

COPY --from=builder /bin/reaction-engine /usr/local/bin/reaction-engine

ENTRYPOINT ["/usr/local/bin/reaction-engine"]
