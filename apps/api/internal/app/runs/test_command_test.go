package runs

import "testing"

func TestEffectiveTestCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		global  string
		project string
		want    string
	}{
		{name: "the project command wins", global: "go test ./...", project: "make check", want: "make check"},
		{name: "an empty project command falls back", global: "go test ./...", project: "", want: "go test ./..."},
		{name: "a blank project command falls back", global: "go test ./...", project: "   ", want: "go test ./..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveTestCommand(tc.global, tc.project); got != tc.want {
				t.Fatalf("effectiveTestCommand(%q, %q) = %q, want %q", tc.global, tc.project, got, tc.want)
			}
		})
	}
}
