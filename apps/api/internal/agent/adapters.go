package agent

// AdapterCommand returns the trusted command for an external agent adapter.
// An unknown or empty adapter returns "" (the configured command is used).
func AdapterCommand(adapter string) string {
	switch adapter {
	case "claude-code":
		// The CLI must exist in the sandbox image; the prompt is the same file
		// every command backend receives.
		return `claude -p "$(cat /workspace/.aip/prompt.txt)" --permission-mode acceptEdits`
	}
	return ""
}
