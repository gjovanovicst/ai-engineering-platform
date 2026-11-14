package runs

import "testing"

func TestParsePluginChecks(t *testing.T) {
	checks, err := ParsePluginChecks(`[{"name":"lint","command":"npm run lint"}]`)
	if err != nil || len(checks) != 1 || checks[0].Name != "lint" {
		t.Fatalf("checks=%v err=%v", checks, err)
	}
	if _, err := ParsePluginChecks(`[{"name":"x"}]`); err == nil {
		t.Fatal("missing command must fail")
	}
	if checks, err := ParsePluginChecks(""); err != nil || len(checks) != 0 {
		t.Fatalf("empty = %v %v", checks, err)
	}
}
