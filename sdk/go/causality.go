package causality

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// Client is the main SDK client for sending events to Causality.
type Client struct {
	config    Config
	batcher   *batcher
	serverCtx *ServerContext
	cancelFn  context.CancelFunc
	doneCh    chan struct{}
	closeOnce sync.Once
}

// New creates a new Causality client with the given configuration.
// The client starts a background goroutine for periodic flushing.
// Call Close() when done to flush remaining events and stop the goroutine.
func New(cfg Config) (*Client, error) {
	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	cfg = cfg.withDefaults()

	// Create transport
	transport := newHTTPTransport(cfg)

	// Create batcher
	batcher := newBatcher(cfg.BatchSize, transport)

	// Collect server context once
	serverCtx := collectServerContext()

	// Create cancellation context for flush loop
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	// Start background flush loop
	go batcher.flushLoop(ctx, cfg.FlushInterval, doneCh)

	return &Client{
		config:    cfg,
		batcher:   batcher,
		serverCtx: serverCtx,
		cancelFn:  cancel,
		doneCh:    doneCh,
	}, nil
}

// Track enqueues an event for asynchronous batch sending.
// It sets the idempotency key, timestamp, app ID, and server context automatically.
// This method is non-blocking and safe for concurrent use.
func (c *Client) Track(event Event) {
	// Generate unique idempotency key
	event.IdempotencyKey = uuid.New().String()

	// Set timestamp if not provided
	if event.Timestamp == "" {
		event.Timestamp = now()
	}

	// Set app ID from config
	event.AppID = c.config.AppID

	// Attach server context
	event.ServerContext = c.serverCtx

	// Add to batch
	if c.batcher.add(event) {
		// Batch is full, trigger async flush
		go func() {
			_ = c.batcher.flush(context.Background())
		}()
	}
}

// Flush synchronously sends all queued events to the server.
// Returns an error if the send fails after all retries.
func (c *Client) Flush() error {
	return c.batcher.flush(context.Background())
}

// Close flushes any remaining events and shuts down the client.
// It stops the background flush goroutine and waits for it to complete.
// Close is safe to call multiple times; subsequent calls are no-ops.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		// Stop the flush loop
		c.cancelFn()

		// Wait for flush loop to exit
		<-c.doneCh

		// Final flush of any remaining events
		err = c.Flush()
	})
	return err
}
