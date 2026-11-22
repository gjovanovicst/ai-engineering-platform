package runs

// Remediable reports the deterministic remediation for an environment failure.
// Only a failure whose fix is mechanical qualifies: the plan is approved and the
// anomaly is the environment, not the change.
func Remediable(failureCode string) (string, bool) {
	switch failureCode {
	case "context_changed":
		// The repository moved after the plan was approved; a fresh run freezes
		// the current context at start.
		return "refreeze", true
	case "dependency_missing":
		// The host-side dependency preparation already ran on the first attempt.
		return "retry", true
	}
	return "", false
}
