// Package workermetrics exposes worker and queue health for Prometheus.
package workermetrics

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Queue reports River job counts.
type Queue interface {
	Stats(ctx context.Context) (available, running int64, oldestSeconds float64, err error)
	Runs(ctx context.Context) (map[string]int64, error)
}

// Handler serves /metrics and /healthz.
type Handler struct {
	Queue   Queue
	Started time.Time
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
		return
	}
	if r.URL.Path != "/metrics" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP aip_worker_up Worker process is up\n# TYPE aip_worker_up gauge\naip_worker_up 1\n")
	_, _ = fmt.Fprintf(w, "# HELP aip_worker_uptime_seconds Worker uptime\n# TYPE aip_worker_uptime_seconds gauge\naip_worker_uptime_seconds %s\n",
		fmt.Sprintf("%.0f", time.Since(h.Started).Seconds()))
	if h.Queue == nil {
		return
	}
	available, running, oldest, err := h.Queue.Stats(r.Context())
	if err != nil {
		_, _ = fmt.Fprintf(w, "# HELP aip_queue_scrape_error Queue scrape failed\n# TYPE aip_queue_scrape_error gauge\naip_queue_scrape_error 1\n")
		return
	}
	_, _ = fmt.Fprintf(w, "# HELP aip_queue_jobs Jobs by state\n# TYPE aip_queue_jobs gauge\n")
	_, _ = fmt.Fprintf(w, "aip_queue_jobs{state=%q} %d\n", "available", available)
	_, _ = fmt.Fprintf(w, "aip_queue_jobs{state=%q} %d\n", "running", running)
	_, _ = fmt.Fprintf(w, "# HELP aip_queue_oldest_seconds Age of the oldest available job\n# TYPE aip_queue_oldest_seconds gauge\naip_queue_oldest_seconds %.3f\n", oldest)
	if runs, rerr := h.Queue.Runs(r.Context()); rerr == nil {
		_, _ = fmt.Fprintf(w, "# HELP aip_runs_total Runs by status (all time)\n# TYPE aip_runs_total gauge\n")
		for status, count := range runs {
			_, _ = fmt.Fprintf(w, "aip_runs_total{status=%q} %d\n", status, count)
		}
	}
}
