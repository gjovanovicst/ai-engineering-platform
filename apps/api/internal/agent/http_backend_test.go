package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPBackendRun(t *testing.T) {
	var gotAuth, gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"line one\nline two"}}],
			"usage":{"prompt_tokens":120,"completion_tokens":30}
		}`))
	}))
	defer server.Close()

	backend := NewHTTPBackend(server.URL+"/v1", "sk-secret", "test-model")
	var lines []string
	result, err := backend.Run(context.Background(), Spec{Prompt: "do it"},
		func(e Event) error {
			if e.Type == EventText {
				lines = append(lines, e.Text)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sk-secret" || gotModel != "test-model" {
		t.Fatalf("auth=%q model=%q", gotAuth, gotModel)
	}
	if len(lines) != 2 || lines[0] != "line one" || !strings.Contains(lines[1], "line two") {
		t.Fatalf("lines = %v", lines)
	}
	if result.InputTokens != 120 || result.OutputTokens != 30 || result.Model != "test-model" {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPBackendRejectsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer server.Close()
	backend := NewHTTPBackend(server.URL, "k", "m")
	if _, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(Event) error { return nil }); err == nil {
		t.Fatal("expected an error for a 401")
	}
}

func TestHTTPBackendRetriesTransientErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer server.Close()

	backend := NewHTTPBackend(server.URL, "k", "m")
	backend.baseDelay = time.Millisecond
	var lines []string
	if _, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(e Event) error {
		if e.Type == EventText {
			lines = append(lines, e.Text)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(lines) != 1 || lines[0] != "ok" {
		t.Fatalf("attempts=%d lines=%v", attempts, lines)
	}
}

func TestHTTPBackendDoesNotRetryBadRequest(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	backend := NewHTTPBackend(server.URL, "k", "m")
	backend.baseDelay = time.Millisecond
	if _, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(Event) error { return nil }); err == nil {
		t.Fatal("expected an error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestHTTPBackendStreamsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, chunk := range []string{
			`{"model":"m","choices":[{"delta":{"content":"line one\n"}}]}`,
			`{"model":"m","choices":[{"delta":{"content":"line two"}}]}`,
			`{"model":"m","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
		} {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	backend := NewHTTPBackend(server.URL, "k", "m")
	var lines []string
	result, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(e Event) error {
		if e.Type == EventText {
			lines = append(lines, e.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "line one" || lines[1] != "line two" {
		t.Fatalf("lines = %v", lines)
	}
	if result.InputTokens != 10 || result.OutputTokens != 5 {
		t.Fatalf("usage = %+v", result)
	}
}
