package httpapi

import "testing"

func TestComponentScaffold(t *testing.T) {
	spec, err := componentScaffold("ButtonGroup", "react", "ds-1")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title == "" || len(spec.Acceptance) != 1 || len(spec.Tags) != 3 {
		t.Fatalf("spec = %+v", spec)
	}
	if _, err := componentScaffold("button", "react", ""); err == nil {
		t.Fatal("lowercase name must fail")
	}
	if _, err := componentScaffold("Button", "svelte", ""); err == nil {
		t.Fatal("unknown kind must fail")
	}
}
