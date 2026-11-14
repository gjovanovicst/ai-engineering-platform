package metrics

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRegistryRendersRequests(t *testing.T) {
	r := New()
	r.Observe("GET", "200", 10*time.Millisecond)
	r.Observe("GET", "200", 20*time.Millisecond)
	var buf bytes.Buffer
	r.Write(&buf, "test")
	out := buf.String()
	for _, want := range []string{"aip_api_up 1", "aip_build_info{version=\"test\"} 1", `aip_http_requests_total{method="GET",status="200"} 2`, "duration_seconds_sum"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
}
