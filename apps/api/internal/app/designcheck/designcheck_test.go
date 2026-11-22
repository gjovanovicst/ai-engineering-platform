package designcheck

import "testing"

func TestScanFlagsRawColoursOnly(t *testing.T) {
	diff := `diff --git a/ui.tsx b/ui.tsx
+++ b/ui.tsx
+const bad = { color: "#4f5eea" };
+const good = { color: "var(--brand)" };
+const good2 = { color: "rgb(var(--brand) / .5)" };
`
	v := Scan(diff, nil)
	if len(v) != 1 || v[0].Path != "ui.tsx" {
		t.Fatalf("violations = %+v", v)
	}
	if v[0].Message == "" {
		t.Fatal("message is empty")
	}
}

func TestSummaryEmpty(t *testing.T) {
	if Summary(nil) != "no design violations" {
		t.Fatal("unexpected summary")
	}
}

func TestScanFlagsForbiddenImports(t *testing.T) {
	diff := `diff --git a/ui.tsx b/ui.tsx
+++ b/ui.tsx
+import { LegacyTable } from "@acme/legacy";
+import { Button } from "@acme/ui";
`
	v := Scan(diff, []string{"@acme/legacy"})
	if len(v) != 1 || v[0].Path != "ui.tsx" {
		t.Fatalf("violations = %+v", v)
	}
}

func TestScanAllowsTheTokenDefinitions(t *testing.T) {
	t.Parallel()
	diff := "+++ b/src/styles/tokens.css\n+  --color-brand: #4f5eea;\n+++ b/src/pages/SignIn.css\n+  color: #ff0000;\n"
	got := Scan(diff, nil)
	if len(got) != 1 || got[0].Path != "src/pages/SignIn.css" {
		t.Fatalf("Scan = %+v, want exactly the SignIn.css raw colour", got)
	}
}

func TestScanSkipsTheTests(t *testing.T) {
	t.Parallel()
	diff := "+++ b/src/styles/tokens.test.ts\n+  '--color-brand': '#4f5eea',\n"
	if got := Scan(diff, nil); len(got) != 0 {
		t.Fatalf("Scan = %+v, want none: a test asserting the token values is not a violation", got)
	}
}
