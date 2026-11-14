package embeddings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("path=%s auth=%s", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0]},{"embedding":[0,1]}]}`))
	}))
	defer srv.Close()
	vectors, err := New(srv.URL, "key", "model").Embed(context.Background(), []string{"a", "b"})
	if err != nil || len(vectors) != 2 || vectors[1][1] != 1 {
		t.Fatalf("vectors=%v err=%v", vectors, err)
	}
}
