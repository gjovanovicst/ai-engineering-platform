package workermetrics

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeQueue struct{}

func (fakeQueue) Stats(context.Context) (int64, int64, float64, error) { return 3, 1, 12.5, nil }
func (fakeQueue) Runs(context.Context) (map[string]int64, error) {
	return map[string]int64{"succeeded": 4, "failed": 1}, nil
}

func TestMetrics(t *testing.T) {
	h := &Handler{Queue: fakeQueue{}, Started: time.Now().Add(-time.Minute)}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{"aip_worker_up 1", `aip_queue_jobs{state="available"} 3`, "aip_queue_oldest_seconds 12.500", `aip_runs_total{status="failed"} 1`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q:\n%s", want, body)
		}
	}
}
