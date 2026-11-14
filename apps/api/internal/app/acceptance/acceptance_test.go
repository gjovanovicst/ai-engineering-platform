package acceptance

import "testing"

func TestCheckRequiresATestForSource(t *testing.T) {
	tests := []struct {
		name string
		diff string
		ok   bool
	}{
		{"source without test", "+++ b/main.go\n+x\n", false},
		{"source with test", "+++ b/main.go\n+x\n+++ b/main_test.go\n+y\n", true},
		{"docs only", "+++ b/README.md\n+x\n", true},
		{"template only", "+++ b/templates/list.html\n+x\n", true},
		{"tsx without test", "+++ b/src/App.tsx\n+x\n", false},
		{"tsx spec", "+++ b/src/App.tsx\n+x\n+++ b/src/App.test.tsx\n+y\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, detail := Check(tt.diff)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v (%s)", ok, tt.ok, detail)
			}
			if detail == "" {
				t.Fatal("detail is empty")
			}
		})
	}
}
