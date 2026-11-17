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

func TestScanIgnoresTestFixturesAndThePragma(t *testing.T) {
	t.Parallel()
	diff := "+++ b/apps/api/internal/httpapi/rbac_test.go\n" +
		"+\treturn fixtureUser{token: \"this-token-was-never-issued\"}\n" +
		"+++ b/apps/api/internal/auth/session_test.go\n" +
		"+\t// aip:allow-secret: generated for this test only\n" +
		"+\ttoken: \"averylongtokenvalue123456\"\n"
	if findings := Scan(diff); len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
}

func TestScanStillFindsRealTokens(t *testing.T) {
	t.Parallel()
	diff := "+++ b/config.go\n+token: \"averylongtokenvalue123456\"\n"
	findings := Scan(diff)
	if len(findings) != 1 || findings[0].Kind != "generic_api_key" {
		t.Fatalf("findings = %+v, want one generic_api_key", findings)
	}
}
