package runs

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// reviewDiffLimit bounds how much of the development diff is sent to the
// review model; larger diffs are truncated with a marker and a caveat. The T-4
// diff was 126 KB: the old 60 KB limit cut off the handler that proves the
// self-approval rule, so the review reported two false majors.
const reviewDiffLimit = 200000

// reviewPrompt frames a review around the development diff. Reviewing the base
// repository instead of the change is a known failure mode (the agent then
// reports that nothing was delivered), so the prompt is explicit about it.
func reviewPrompt(diff string) string {
	var b strings.Builder
	b.WriteString("\nYou are the REVIEW agent. Do NOT modify files.\n")
	diff = strings.TrimSpace(diff)
	if diff == "" {
		b.WriteString("No development diff was found for this task. Report a blocker finding " +
			"stating that nothing was delivered; do not review the base repository as the change.\n")
		return b.String()
	}
	files := changedFiles(diff)
	truncated := len(diff) > reviewDiffLimit
	if truncated {
		diff = diff[:reviewDiffLimit] + "\n[diff truncated]\n"
	}
	if len(files) > 0 {
		b.WriteString("Changed files (" + strconv.Itoa(len(files)) + "):\n")
		for _, file := range files {
			b.WriteString("- " + file + "\n")
		}
		b.WriteString("\n")
	}
	if truncated {
		b.WriteString("The diff below is truncated at the review limit. Do not report files or hunks " +
			"that are not shown, and do not claim that a file is missing when it may be past the limit.\n\n")
	}
	b.WriteString("The change under review is the latest development run's diff. Review THESE changes; " +
		"the base repository alone is not the change. Flag hand-written cryptography, " +
		"reimplemented third-party libraries and agent-authored vendor code as findings.\n\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n")
	return b.String()
}

// reviewBlockers counts the findings a human marked as blockers: only they
// make a pull request require a fix before the merge.
func reviewBlockers(raw []byte) int {
	var payload struct {
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	n := 0
	for _, f := range payload.Findings {
		if strings.EqualFold(f.Severity, "blocker") {
			n++
		}
	}
	return n
}

// changedFiles lists the paths in a diff, in order.
func changedFiles(diff string) []string {
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "diff --git a/") {
			continue
		}
		rest := strings.TrimPrefix(line, "diff --git a/")
		if idx := strings.Index(rest, " b/"); idx > 0 {
			files = append(files, rest[:idx])
		}
	}
	return files
}

// developmentDiff returns the latest development diff for the run's task.
func (w *JobWorker) developmentDiff(ctx context.Context, run domain.Run) string {
	dev, err := w.store.LatestRun(ctx, run.OrgID, run.TaskID, string(domain.AgentDevelopment))
	if err != nil || len(dev.Diff) == 0 {
		return ""
	}
	return string(dev.Diff)
}
