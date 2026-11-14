package runs

import "testing"

func TestSymbolsExtractsGoSymbols(t *testing.T) {
	archive := tarFilesForMap(map[string]string{
		"pagination.go": "package main\nfunc Page() {}\ntype Pager struct{}\nfunc (p Pager) Next() {}\n",
	})
	lines := Symbols(archive, []string{"pagination.go"}, 10)
	if len(lines) != 1 {
		t.Fatalf("lines = %v", lines)
	}
	for _, want := range []string{"Page()", "type Pager", "Pager) Next()"} {
		if !contains(lines[0], want) {
			t.Fatalf("%q missing %q", lines[0], want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
