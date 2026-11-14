package agent

import (
	"strings"
	"testing"
)

func TestAdapterCommand(t *testing.T) {
	if got := AdapterCommand(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
	got := AdapterCommand("claude-code")
	if !strings.Contains(got, "claude -p") || !strings.Contains(got, "/workspace/.aip/prompt.txt") {
		t.Fatalf("claude-code = %q", got)
	}
	if got := AdapterCommand("unknown"); got != "" {
		t.Fatalf("unknown = %q", got)
	}
}
