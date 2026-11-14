package embeddings

import "context"

// CachedClient implements the Embedder interface with a vector cache.
type CachedClient struct {
	Client *Client
	Cache  Cache
}

// Embed returns cached vectors and fills misses.
func (c CachedClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	return EmbedCached(ctx, c.Client, c.Cache, inputs)
}
