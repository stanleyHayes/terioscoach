// Package mongodb is the outbound adapter for MongoDB Atlas. It owns the
// driver client and index management; repository implementations arrive in
// later backend tasks and build on this foundation.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Default timeouts for connection lifecycle operations.
const (
	connectTimeout    = 10 * time.Second
	pingTimeout       = 5 * time.Second
	disconnectTimeout = 10 * time.Second
)

// Client wraps the driver client with bounded connect/ping/disconnect
// operations. It is safe to share across adapters.
type Client struct {
	client *mongo.Client
}

// Connect dials MongoDB and verifies the connection with a ping before
// returning. Callers own the returned Client and must Disconnect on shutdown.
// Note: the driver v2 Connect is lazy, so the ping below is what actually
// bounds the dial; callers should pass an already-deadlined ctx.
func Connect(ctx context.Context, uri string) (*Client, error) {
	mc, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongodb connect: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, connectTimeout)
	defer pingCancel()
	if err := mc.Ping(pingCtx, nil); err != nil {
		_ = mc.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb ping after connect: %w", err)
	}

	return &Client{client: mc}, nil
}

// Ping reports whether the cluster is reachable; used as the readiness check.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := c.client.Ping(pingCtx, nil); err != nil {
		return fmt.Errorf("mongodb ping: %w", err)
	}
	return nil
}

// Database returns a handle to the application database.
func (c *Client) Database(name string) *mongo.Database {
	return c.client.Database(name)
}

// Disconnect releases the connection pool with a bounded timeout.
func (c *Client) Disconnect(ctx context.Context) error {
	disconnectCtx, cancel := context.WithTimeout(ctx, disconnectTimeout)
	defer cancel()
	if err := c.client.Disconnect(disconnectCtx); err != nil {
		return fmt.Errorf("mongodb disconnect: %w", err)
	}
	return nil
}
