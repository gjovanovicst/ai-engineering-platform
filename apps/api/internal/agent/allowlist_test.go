package agent

import "testing"

func TestToolAllowlistHidesWriteForReadOnlyAgents(t *testing.T) {
	names := func(readOnly bool) map[string]bool {
		out := map[string]bool{}
		for _, spec := range toolSpecs(readOnly) {
			fn := spec["function"].(map[string]any)
			out[fn["name"].(string)] = true
		}
		return out
	}
	readOnly := names(true)
	if readOnly["write_file"] {
		t.Fatal("read-only agents must not see write_file")
	}
	for _, want := range []string{"read_file", "list_files", "grep", "run_tests"} {
		if !readOnly[want] {
			t.Fatalf("read-only tools missing %q", want)
		}
	}
	if !names(false)["write_file"] {
		t.Fatal("development agents need write_file")
	}
}
