// Package nats provides NATS JetStream integration for event publishing and streaming.
package nats

import (
	"time"
)

// Config holds NATS connection and stream configuration.
type Config struct {
	// URL is the NATS server URL (e.g., "nats://localhost:4222")
	URL string `env:"NATS_URL" envDefault:"nats://localhost:4222"`

	// Name is the client connection name for monitoring
	Name string `env:"NATS_CLIENT_NAME" envDefault:"causality-server"`

	// MaxReconnects is the maximum number of reconnection attempts
	MaxReconnects int `env:"NATS_MAX_RECONNECTS" envDefault:"60"`

	// ReconnectWait is the time to wait between reconnection attempts
	ReconnectWait time.Duration `env:"NATS_RECONNECT_WAIT" envDefault:"2s"`

	// Timeout is the connection timeout
	Timeout time.Duration `env:"NATS_TIMEOUT" envDefault:"5s"`

	// Stream configuration
	Stream StreamConfig `envPrefix:"NATS_STREAM_"`
}

// StreamConfig holds JetStream stream configuration.
type StreamConfig struct {
	// Name is the stream name
	Name string `env:"NAME" envDefault:"CAUSALITY_EVENTS"`

	// Subjects are the subjects to capture
	Subjects []string `env:"SUBJECTS" envDefault:"events.>,requests.>,responses.>,anomalies.>"`

	// MaxAge is the maximum age of messages in the stream
	MaxAge time.Duration `env:"MAX_AGE" envDefault:"168h"` // 7 days

	// MaxBytes is the maximum size of the stream in bytes
	MaxBytes int64 `env:"MAX_BYTES" envDefault:"1073741824"` // 1GB

	// Replicas is the number of replicas for the stream
	Replicas int `env:"REPLICAS" envDefault:"1"`

	// Storage is the storage type (file or memory)
	Storage string `env:"STORAGE" envDefault:"file"`
}

// ConsumerConfig holds JetStream consumer configuration.
type ConsumerConfig struct {
	// Name is the consumer durable name
	Name string

	// FilterSubject is the subject filter for the consumer
	FilterSubject string

	// AckWait is the time to wait for acknowledgment
	AckWait time.Duration

	// MaxAckPending is the maximum number of pending acknowledgments
	MaxAckPending int

	// MaxDeliver is the maximum number of delivery attempts
	MaxDeliver int
}

// DefaultConsumerConfigs returns the default consumer configurations.
func DefaultConsumerConfigs() []ConsumerConfig {
	return []ConsumerConfig{
		{
			Name:          "warehouse-sink",
			FilterSubject: ">",
			AckWait:       30 * time.Second,
			MaxAckPending: 10000,
			MaxDeliver:    5,
		},
		{
			Name:          "analysis-engine",
			FilterSubject: "events.>",
			AckWait:       10 * time.Second,
			MaxAckPending: 1000,
			MaxDeliver:    3,
		},
		{
			Name:          "alerting",
			FilterSubject: "anomalies.>",
			AckWait:       5 * time.Second,
			MaxAckPending: 100,
			MaxDeliver:    3,
		},
	}
}
