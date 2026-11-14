// Package s3cache stores embedding vectors in an S3-compatible bucket.
package s3cache

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/embeddings"
)

// Objects is the subset of the artifacts store the cache needs.
type Objects interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// Cache implements embeddings.Cache.
type Cache struct{ objects Objects }

// New wires the cache.
func New(objects Objects) *Cache { return &Cache{objects: objects} }

func (c *Cache) Get(ctx context.Context, key string) ([]float32, error) {
	raw, err := c.objects.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var vector []float32
	if err := json.Unmarshal(raw, &vector); err != nil {
		return nil, err
	}
	return vector, nil
}

func (c *Cache) Set(ctx context.Context, key string, vector []float32) error {
	raw, err := json.Marshal(vector)
	if err != nil {
		return err
	}
	return c.objects.Put(ctx, key, bytes.NewReader(raw), int64(len(raw)), "application/json")
}

var _ embeddings.Cache = (*Cache)(nil)
