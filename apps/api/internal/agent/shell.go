package agent

// toolchainShell wraps a command so a login shell cannot hide the image's
// toolchain: /etc/profile resets PATH and drops /usr/local/go/bin, which made
// the test gate die with "go: not found" although the sandbox env carried PATH.
func toolchainShell(script string) []string {
	return []string{"sh", "-c", `export PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin:$PATH; ` + script}
}
