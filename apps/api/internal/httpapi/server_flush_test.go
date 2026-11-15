package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type flushCounter struct {
	http.ResponseWriter
	flushed int
}

func (f *flushCounter) Flush() { f.flushed++ }

func TestStatusRecorderPreservesFlusher(t *testing.T) {
	t.Parallel()
	spy := &flushCounter{ResponseWriter: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: spy, status: http.StatusOK}

	flusher, ok := any(rec).(http.Flusher)
	if !ok {
		t.Fatal("statusRecorder must implement http.Flusher; the run event stream would never flush")
	}
	flusher.Flush()
	if spy.flushed != 1 {
		t.Fatalf("flushes = %d, want the call to reach the underlying writer", spy.flushed)
	}
}

func TestStatusRecorderFlushWithoutUnderlyingFlusher(t *testing.T) {
	t.Parallel()
	plain := httptest.NewRecorder() // httptest.ResponseRecorder implements Flusher, so use a bare writer
	rec := &statusRecorder{ResponseWriter: struct{ http.ResponseWriter }{plain}}
	flusher, ok := any(rec).(http.Flusher)
	if !ok {
		t.Fatal("statusRecorder must implement http.Flusher")
	}
	flusher.Flush() // must not panic when the wrapped writer cannot flush
}
