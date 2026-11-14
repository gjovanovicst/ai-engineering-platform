package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a fixed-window limiter shared by API replicas.
type Redis struct {
	client *redis.Client
	limit  int
	window time.Duration
	prefix string
}

// NewRedis returns a shared limiter. Keys are namespaced by prefix.
func NewRedis(client *redis.Client, limit int, window time.Duration, prefix string) *Redis {
	return &Redis{client: client, limit: limit, window: window, prefix: prefix}
}

var incrScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`)

// Allow reports whether the event is permitted. Failures fail open so an auth
// outage does not lock every user out; callers still have the local fallback
// semantics for a single instance.
func (r *Redis) Allow(key string) (bool, time.Duration) {
	if r == nil || r.client == nil {
		return true, 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := incrScript.Run(ctx, r.client, []string{r.prefix + ":" + key}, r.window.Milliseconds()).Slice()
	if err != nil || len(result) != 2 {
		return true, 0
	}
	count, _ := result[0].(int64)
	ttl, _ := result[1].(int64)
	if count > int64(r.limit) {
		return false, time.Duration(ttl) * time.Millisecond
	}
	return true, 0
}
