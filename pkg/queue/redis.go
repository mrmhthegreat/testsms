// pkg/queue/redis.go — Redis client wrapper for the SMS queue.
package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	SMSQueue = "sms_queue"
	StatusTTL = time.Hour
)

// Client wraps a redis.Client for queue operations.
type Client struct {
	rdb *redis.Client
}

// New creates a new Redis-backed queue client.
func New(addr string) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &Client{rdb: rdb}
}

// Ping verifies the Redis connection.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Enqueue pushes a job ID to the right of the named list.
func (c *Client) Enqueue(ctx context.Context, list, value string) error {
	return c.rdb.RPush(ctx, list, value).Err()
}

// Dequeue blocks until a job is available and pops from the left.
// Returns the list name and value, or an error on timeout/disconnect.
func (c *Client) Dequeue(ctx context.Context, list string, timeout time.Duration) (string, error) {
	result, err := c.rdb.BLPop(ctx, timeout, list).Result()
	if err != nil {
		return "", err
	}
	// result[0] = list name, result[1] = value
	return result[1], nil
}

// Set stores a string value with the default TTL.
func (c *Client) Set(ctx context.Context, key, value string) error {
	return c.rdb.Set(ctx, key, value, StatusTTL).Err()
}

// Get retrieves a string value for a key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}
