// Package acceptance checks that a diff proves its behaviour with tests.
package acceptance

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var (
	sourceExt = map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rb": true, ".java": true, ".kt": true, ".rs": true,
		".cs": true, ".php": true,
	}
	testFile = regexp.MustCompile(`(?i)(_test\.go|\.test\.[tj]sx?|\.spec\.[tj]sx?|(^|/)test_.*\.py$|_test\.py$|Tests?\.(java|kt|cs)$)`)
)

// Check reports whether a mixed diff touches source without touching a test.
// A diff with no source files (docs, styles, templates) passes: there is
// nothing executable to prove.
func Check(diff string) (bool, string) {
	var source, tests []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++ b/") {
			continue
		}
		file := strings.TrimPrefix(line, "+++ b/")
		ext := strings.ToLower(path.Ext(file))
		if testFile.MatchString(file) {
			tests = append(tests, file)
			continue
		}
		if sourceExt[ext] {
			source = append(source, file)
		}
	}
	if len(source) == 0 {
		return true, "no executable source changed"
	}
	if len(tests) == 0 {
		return false, fmt.Sprintf("source changed (%s) but no test file was added or updated", strings.Join(source, ", "))
	}
	return true, fmt.Sprintf("%d source file(s) proven by %s", len(source), strings.Join(tests, ", "))
}
