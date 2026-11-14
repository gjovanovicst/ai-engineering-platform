package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// Cache stores vectors by key (S3, Redis, memory).
type Cache interface {
	Get(ctx context.Context, key string) ([]float32, error)
	Set(ctx context.Context, key string, vector []float32) error
}

// CacheKey is deterministic for a model and input text.
func CacheKey(model, text string) string {
	sum := sha256.Sum256([]byte(model + "\x00" + text))
	return "embeddings/" + hex.EncodeToString(sum[:])
}

// EmbedCached returns cached vectors where possible and fills the cache for the
// rest. A cache error degrades to a normal embed (never fails the run twice).
func EmbedCached(ctx context.Context, client *Client, cache Cache, inputs []string) ([][]float32, error) {
	out := make([][]float32, len(inputs))
	var missing []int
	var texts []string
	for i, input := range inputs {
		if cache == nil {
			missing = append(missing, i)
			texts = append(texts, input)
			continue
		}
		if vector, err := cache.Get(ctx, CacheKey(client.Model, input)); err == nil && len(vector) > 0 {
			out[i] = vector
			continue
		}
		missing = append(missing, i)
		texts = append(texts, input)
	}
	if len(missing) == 0 {
		return out, nil
	}
	vectors, err := client.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	for j, index := range missing {
		out[index] = vectors[j]
		if cache != nil {
			_ = cache.Set(ctx, CacheKey(client.Model, texts[j]), vectors[j])
		}
	}
	return out, nil
}
