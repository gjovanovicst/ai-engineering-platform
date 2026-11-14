// Package replay prevents reuse of SAML assertion ids.
package replay

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache records seen ids until their expiry.
type Cache interface {
	Seen(ctx context.Context, id string, until time.Time) (bool, error)
}

// Memory is a process-local fallback.
type Memory struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// NewMemory returns a process-local cache.
func NewMemory() *Memory { return &Memory{seen: map[string]time.Time{}} }

// Seen reports whether id was already recorded.
func (m *Memory) Seen(_ context.Context, id string, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for key, expiry := range m.seen {
		if expiry.Before(now) {
			delete(m.seen, key)
		}
	}
	if _, ok := m.seen[id]; ok {
		return true, nil
	}
	m.seen[id] = until
	return false, nil
}

// Redis is a shared replay cache across API replicas.
type Redis struct{ client *redis.Client }

// NewRedis returns a shared cache.
func NewRedis(client *redis.Client) *Redis { return &Redis{client: client} }

// Seen reports whether id was already recorded, setting it with a TTL.
func (r *Redis) Seen(ctx context.Context, id string, until time.Time) (bool, error) {
	ttl := time.Until(until)
	if ttl <= 0 {
		ttl = time.Second
	}
	ok, err := r.client.SetNX(ctx, "aip:saml:replay:"+id, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return !ok, nil
}
