// Package designcheck scans a diff for violations of the design standard.
package designcheck

import (
	"fmt"
	"regexp"
	"strings"
)

// Violation is one non-compliant added line.
type Violation struct {
	Path    string `json:"path"`
	Line    string `json:"line"`
	Message string `json:"message"`
}

var (
	hexColor = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	fileLine = regexp.MustCompile(`^\+\+\+ b/(.+)$`)
	// tokenDefinition is the one place a raw value belongs: the tokens file
	// declares --color-brand: #4f5eea; it is not a violation.
	tokenDefinition = regexp.MustCompile(`^\s*--[a-zA-Z0-9-]+\s*:`)
)

// isTestPath reports a test file: a test that asserts the token values is not
// a design violation either.
func isTestPath(path string) bool {
	return strings.Contains(path, "_test.") || strings.Contains(path, ".test.") || strings.Contains(path, ".spec.")
}

// Scan checks added lines for raw colour values. Lines that use a design token
// (var(--…) / rgb(var(…))) are allowed; everything else is a violation.
func Scan(diff string, forbidden []string) []Violation {
	var out []Violation
	path := ""
	for _, raw := range strings.Split(diff, "\n") {
		if match := fileLine.FindStringSubmatch(raw); match != nil {
			path = match[1]
			continue
		}
		if !strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "+++") {
			continue
		}
		line := strings.TrimPrefix(raw, "+")
		if isTestPath(path) {
			continue
		}
		// The token definitions and the line that sets a custom property are the
		// contract's source, not a violation; only a raw value used in a screen is.
		if tokenDefinition.MatchString(line) {
			continue
		}
		for _, imp := range forbidden {
			if imp != "" && strings.Contains(line, imp) {
				out = append(out, Violation{Path: path, Line: strings.TrimSpace(line), Message: "forbidden or deprecated component import: " + imp})
				break
			}
		}
		if len(out) > 0 && out[len(out)-1].Path == path && strings.Contains(line, out[len(out)-1].Message) {
			if len(out) >= 20 {
				return out
			}
			continue
		}
		if !hexColor.MatchString(line) || strings.Contains(line, "var(--") || strings.Contains(line, "rgb(var(") {
			if len(out) >= 20 {
				break
			}
			continue
		}
		out = append(out, Violation{Path: path, Line: strings.TrimSpace(line), Message: "raw colour value; use a design token instead"})
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// Summary renders violations for a test result.
func Summary(violations []Violation) string {
	if len(violations) == 0 {
		return "no design violations"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d design violation(s):\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", v.Path, v.Message, v.Line)
	}
	b.WriteString("Use the project's design tokens and approved components.\n")
	return b.String()
}
