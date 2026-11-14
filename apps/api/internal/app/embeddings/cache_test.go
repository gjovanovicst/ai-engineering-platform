package embeddings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memoryCache struct{ data map[string][]float32 }

func (m *memoryCache) Get(_ context.Context, key string) ([]float32, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return nil, context.Canceled
}
func (m *memoryCache) Set(_ context.Context, key string, vector []float32) error {
	m.data[key] = vector
	return nil
}

func TestEmbedCachedUsesAndFillsCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`))
	}))
	defer srv.Close()
	client := New(srv.URL, "", "m")
	cache := &memoryCache{data: map[string][]float32{}}
	for i := 0; i < 2; i++ {
		if _, err := EmbedCached(context.Background(), client, cache, []string{"same"}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}
