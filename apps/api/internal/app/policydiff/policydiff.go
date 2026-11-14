// Package policydiff evaluates a diff against project policy.
package policydiff

import (
	"fmt"
	"strings"
)

// Violations returns human-readable policy violations.
func Violations(diff string, maxBytes int, forbidden []string) []string {
	var out []string
	if maxBytes > 0 && len(diff) > maxBytes {
		out = append(out, fmt.Sprintf("diff is %d bytes; policy limit is %d", len(diff), maxBytes))
	}
	for _, file := range Files(diff) {
		for _, blocked := range forbidden {
			if file == blocked || strings.HasPrefix(file, strings.TrimSuffix(blocked, "/")+"/") {
				out = append(out, "forbidden path changed: "+file)
			}
		}
	}
	return out
}

// Files extracts changed paths from a unified diff.
func Files(diff string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++ b/") {
			continue
		}
		path := strings.TrimPrefix(line, "+++ b/")
		if path != "" && !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}
