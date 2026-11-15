package runs

import (
	"context"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// reviewDiffLimit bounds how much of the development diff is sent to the
// review model; larger diffs are truncated with a marker.
const reviewDiffLimit = 60000

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
	if len(diff) > reviewDiffLimit {
		diff = diff[:reviewDiffLimit] + "\n[diff truncated]\n"
	}
	b.WriteString("The change under review is the latest development run's diff. Review THESE changes; " +
		"the base repository alone is not the change.\n\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n")
	return b.String()
}

// developmentDiff returns the latest development diff for the run's task.
func (w *JobWorker) developmentDiff(ctx context.Context, run domain.Run) string {
	dev, err := w.store.LatestRun(ctx, run.OrgID, run.TaskID, string(domain.AgentDevelopment))
	if err != nil || len(dev.Diff) == 0 {
		return ""
	}
	return string(dev.Diff)
}
