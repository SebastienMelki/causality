package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client wraps NATS connection and JetStream context.
type Client struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	config Config
	logger *slog.Logger
}

// NewClient creates a new NATS client with the given configuration.
func NewClient(ctx context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With("component", "nats-client")

	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.Timeout(cfg.Timeout),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("disconnected from NATS", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("reconnected to NATS", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", "error", err)
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &Client{
		conn:   conn,
		js:     js,
		config: cfg,
		logger: logger,
	}

	logger.Info("connected to NATS",
		"url", conn.ConnectedUrl(),
		"server_id", conn.ConnectedServerId(),
	)

	return client, nil
}

// JetStream returns the JetStream context.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Conn returns the underlying NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// IsConnected returns true if the client is connected to NATS.
func (c *Client) IsConnected() bool {
	return c.conn.IsConnected()
}

// Status returns the connection status.
func (c *Client) Status() nats.Status {
	return c.conn.Status()
}

// Drain gracefully drains the connection.
func (c *Client) Drain() error {
	return c.conn.Drain()
}

// Close closes the NATS connection.
func (c *Client) Close() {
	c.conn.Close()
}

// HealthCheck performs a health check on the NATS connection.
func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.conn.IsConnected() {
		return fmt.Errorf("NATS is not connected, status: %s", c.conn.Status())
	}

	// Check JetStream availability
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := c.js.AccountInfo(ctx)
	if err != nil {
		return fmt.Errorf("JetStream health check failed: %w", err)
	}

	return nil
}
