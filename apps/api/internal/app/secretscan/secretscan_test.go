package secretscan

import "testing"

func TestScanFindsAndIgnoresPlaceholders(t *testing.T) {
	diff := "+++ b/config.go\n+const key = \"" + "AKIA" + "ABCDEFGHIJKLMNOP" + "\"\n+const example = \"your_api_key_here\"\n"
	findings := Scan(diff)
	if len(findings) != 1 || findings[0].Kind != "aws_access_key" {
		t.Fatalf("findings = %+v", findings)
	}
	if Summary(nil) != "no secrets detected" {
		t.Fatal("unexpected summary")
	}
}
