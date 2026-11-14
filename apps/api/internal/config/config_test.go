package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadRequiresDatabaseDSN(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "")
	t.Setenv("AIP_ENV", "local")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded without AIP_DB_DSN, want error")
	}
	if !strings.Contains(err.Error(), "AIP_DB_DSN") {
		t.Fatalf("error does not mention AIP_DB_DSN: %v", err)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "postgres://localhost/aip")
	t.Setenv("AIP_ENV", "local")
	t.Setenv("AIP_HTTP_ADDR", "")
	t.Setenv("AIP_ACCESS_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v, want 15m", cfg.AccessTTL)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "postgres://localhost/aip")
	t.Setenv("AIP_ACCESS_TTL", "fifteen-minutes")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with invalid duration, want error")
	}
}

func TestRedactedHidesDSN(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "postgres://user:secret@localhost/aip")
	t.Setenv("AIP_ENV", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Redacted(); strings.Contains(got, "secret") {
		t.Fatalf("Redacted() leaks the password: %s", got)
	}
}

func TestGitHubTokenEnv(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "postgres://localhost/aip")
	t.Setenv("AIP_GITHUB_TOKEN", "github_pat_test")
	t.Setenv("GITHUB_TOKEN", "legacy")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHubToken != "github_pat_test" {
		t.Fatalf("token = %q, want the AIP_ variable", cfg.GitHubToken)
	}
}

func TestLoadRejectsUnimplementedEgressAllowlist(t *testing.T) {
	t.Setenv("AIP_DB_DSN", "postgres://localhost/aip")
	t.Setenv("AIP_ENV", "local")
	t.Setenv("AIP_SANDBOX_EGRESS_ALLOW", "proxy.golang.org")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "AIP_SANDBOX_EGRESS_ALLOW") {
		t.Fatalf("err = %v, want a fail-closed egress error", err)
	}
}
