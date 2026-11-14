package figma

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComponents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Figma-Token") != "tok" {
			t.Errorf("token = %q", r.Header.Get("X-Figma-Token"))
		}
		_, _ = w.Write([]byte(`{"document":{"id":"0","type":"DOCUMENT","children":[{"id":"1","name":"Button","type":"COMPONENT"},{"id":"2","name":"Card","type":"COMPONENT"}]}}`))
	}))
	defer srv.Close()
	client := New(srv.URL, "tok")
	components, err := client.Components(context.Background(), "file")
	if err != nil || len(components) != 2 || components[0].Name != "Button" {
		t.Fatalf("components=%v err=%v", components, err)
	}
}
