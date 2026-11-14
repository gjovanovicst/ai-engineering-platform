// Package secretscan finds credentials accidentally added by an agent.
package secretscan

import (
	"regexp"
	"strings"
)

var patterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"aws_access_key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"github_token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`)},
	{"github_pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{40,}`)},
	{"slack_token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
	{"private_key", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
	{"generic_api_key", regexp.MustCompile(`(?i)(api[_-]?key|secret|token)\s*[:=]\s*["'][A-Za-z0-9_\-]{20,}["']`)},
}

// Finding is one likely secret in a diff.
type Finding struct {
	Path string
	Kind string
	Line string
}

// Scan inspects added diff lines. Placeholders and documented examples are ignored.
func Scan(diff string) []Finding {
	var out []Finding
	path := ""
	for _, raw := range strings.Split(diff, "\n") {
		if strings.HasPrefix(raw, "+++ b/") {
			path = strings.TrimPrefix(raw, "+++ b/")
			continue
		}
		if !strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "+++") {
			continue
		}
		line := strings.TrimSpace(strings.TrimPrefix(raw, "+"))
		lower := strings.ToLower(line)
		if strings.Contains(lower, "example") || strings.Contains(lower, "redacted") ||
			strings.Contains(lower, "placeholder") || strings.Contains(lower, "xxx") ||
			strings.Contains(lower, "your_") || strings.Contains(lower, "changeme") {
			continue
		}
		for _, pattern := range patterns {
			if pattern.re.MatchString(line) {
				out = append(out, Finding{Path: path, Kind: pattern.name, Line: truncate(line, 160)})
				break
			}
		}
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Summary renders findings for a check.
func Summary(findings []Finding) string {
	if len(findings) == 0 {
		return "no secrets detected"
	}
	var b strings.Builder
	b.WriteString("possible secrets in the diff:\n")
	for _, finding := range findings {
		b.WriteString("- " + finding.Path + ": " + finding.Kind + " (" + finding.Line + ")\n")
	}
	b.WriteString("Remove the credential, rotate it if it was real, and use the secrets store.\n")
	return b.String()
}
