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
	if readOnly["fetch_module"] {
		t.Fatal("read-only agents must not fetch modules")
	}
	for _, want := range []string{"read_file", "list_files", "grep", "run_tests"} {
		if !readOnly[want] {
			t.Fatalf("read-only tools missing %q", want)
		}
	}
	if !names(false)["write_file"] {
		t.Fatal("development agents need write_file")
	}
	if !names(false)["fetch_module"] {
		t.Fatal("development agents need fetch_module")
	}
	if names(true)["fetch_module"] {
		t.Fatal("read-only agents must not fetch modules")
	}
}
