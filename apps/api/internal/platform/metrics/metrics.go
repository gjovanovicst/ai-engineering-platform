// Package metrics keeps a tiny Prometheus text registry for the API.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"
)

type key struct {
	Method string
	Status string
}

// Registry is a concurrency-safe request metrics registry.
type Registry struct {
	mu        sync.Mutex
	requests  map[key]uint64
	durations map[key]float64
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{requests: map[key]uint64{}, durations: map[key]float64{}}
}

// Observe records one completed request.
func (r *Registry) Observe(method, status string, d time.Duration) {
	if r == nil {
		return
	}
	k := key{Method: method, Status: status}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[k]++
	r.durations[k] += d.Seconds()
}

// Write renders Prometheus text format.
func (r *Registry) Write(w io.Writer, version string) {
	_, _ = fmt.Fprintf(w, "# HELP aip_api_up API process is up\n# TYPE aip_api_up gauge\naip_api_up 1\n")
	_, _ = fmt.Fprintf(w, "# HELP aip_build_info Build information\n# TYPE aip_build_info gauge\n")
	_, _ = fmt.Fprintf(w, "aip_build_info{version=%q} 1\n", version)
	if r == nil {
		return
	}
	r.mu.Lock()
	keys := make([]key, 0, len(r.requests))
	for k := range r.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method == keys[j].Method {
			return keys[i].Status < keys[j].Status
		}
		return keys[i].Method < keys[j].Method
	})
	type row struct {
		k key
		n uint64
		d float64
	}
	rows := make([]row, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, row{k: k, n: r.requests[k], d: r.durations[k]})
	}
	r.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP aip_http_requests_total HTTP requests by method and status")
	_, _ = fmt.Fprintln(w, "# TYPE aip_http_requests_total counter")
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "aip_http_requests_total{method=%q,status=%q} %d\n", row.k.Method, row.k.Status, row.n)
	}
	_, _ = fmt.Fprintln(w, "# HELP aip_http_request_duration_seconds_sum Total request duration")
	_, _ = fmt.Fprintln(w, "# TYPE aip_http_request_duration_seconds_sum counter")
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "aip_http_request_duration_seconds_sum{method=%q,status=%q} %s\n", row.k.Method, row.k.Status, strconv.FormatFloat(row.d, 'f', 6, 64))
	}
}
