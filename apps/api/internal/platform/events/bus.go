// Package events publishes and subscribes to run events over Redis pub/sub.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Bus is a thin Redis pub/sub wrapper. A nil Bus means "not configured".
type Bus struct {
	client *redis.Client
}

// New connects; an empty address returns (nil, nil) so callers fall back to
// database polling.
func New(ctx context.Context, addr string) (*Bus, error) {
	if addr == "" {
		return nil, nil
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("events: redis ping: %w", err)
	}
	return &Bus{client: client}, nil
}

// Channel returns the per-run channel name.
func Channel(runID string) string { return "run:" + runID }

// Publish sends a payload to a channel.
func (b *Bus) Publish(ctx context.Context, channel string, payload []byte) error {
	if b == nil {
		return nil
	}
	return b.client.Publish(ctx, channel, payload).Err()
}

// Subscribe returns a channel of payloads and a cancel function. The returned
// channel is closed when the subscription stops.
func (b *Bus) Subscribe(ctx context.Context, channel string) (<-chan []byte, func(), error) {
	if b == nil {
		return nil, func() {}, fmt.Errorf("events: redis is not configured")
	}
	sub := b.client.Subscribe(ctx, channel)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, func() {}, fmt.Errorf("events: subscribe: %w", err)
	}
	out := make(chan []byte, 32)
	done := make(chan struct{})
	go func() {
		defer close(out)
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				select {
				case out <- []byte(msg.Payload):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	cancel := func() {
		close(done)
		_ = sub.Close()
	}
	return out, cancel, nil
}

// Close releases the client.
func (b *Bus) Close() error {
	if b == nil {
		return nil
	}
	return b.client.Close()
}
