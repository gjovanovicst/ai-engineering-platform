package runs

import "github.com/gjovanovicst/ai-engineering-platform/internal/app/policydiff"

// policyViolation reports the first diff-size or forbidden-path violation.
func policyViolation(diff string, maxBytes int, forbidden []string) string {
	if violations := policydiff.Violations(diff, maxBytes, forbidden); len(violations) > 0 {
		return violations[0]
	}
	return ""
}
