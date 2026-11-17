package runs

import (
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// canRetry guards the single structured-output retry against runaway cost.
func canRetry(run domain.Run, result agent.Result) bool {
	if run.Budget.TokensOut <= 0 {
		return true
	}
	return result.OutputTokens*2 < run.Budget.TokensOut
}

func retryCorrection(reason string) string {
	switch reason {
	case "plan_missing":
		return "\n\nYour previous answer did not include the required structured plan. " +
			"Finish with exactly one line: AIP_PLAN: {\"steps\":[...],\"files\":[...],\"risks\":[...]}"
	case "proof_missing":
		return "\n\nYour previous answer did not include AIP_PROOF. Map every acceptance criterion " +
			"to a test you added or changed, or to a manual reason when automation is not practical. " +
			"Finish with exactly one line: AIP_PROOF: {\"evidence\":[{\"criterion\":\"AC1\",\"kind\":\"automated\",\"test\":\"TestName\",\"files\":[\"path_test.go\"]}]}"
	case "infra_test_failure":
		return "\n\nThe test command reported an infrastructure failure (network, timeout, sandbox). " +
			"Re-run the tests and finish normally; do not weaken the tests."
	case "flaky_test_failure":
		return "\n\nThe test command reported a flaky/intermittent failure. " +
			"Re-run the tests once and finish normally; do not weaken the tests."
	default:
		return ""
	}
}

// classifyTestFailure is deliberately conservative: unknown failures are code.
func classifyTestFailure(output string) string {
	lower := strings.ToLower(output)
	// A missing dependency is not a test failure the agent can fix by retrying:
	// the sandbox is offline by design and the platform has to prepare it.
	for _, marker := range []string{
		"no required module provides package", "missing go.sum entry",
		"cannot find module providing package", "cannot find package",
		"module lookup disabled", "no matching versions for query",
		"npm err! code enotfound", "npm err! network", "getaddrinfo enotfound",
		"could not resolve dependency",
	} {
		if strings.Contains(lower, marker) {
			return "dependency"
		}
	}
	for _, marker := range []string{"flaky", "flaked", "intermittent"} {
		if strings.Contains(lower, marker) {
			return "flaky"
		}
	}
	for _, marker := range []string{
		"connection refused", "no such host", "temporary failure in name resolution",
		"network is unreachable", "i/o timeout", "context deadline exceeded",
		"docker daemon", "cannot connect to the docker daemon", "sandbox", "oomkilled",
		"no space left on device", "tls handshake timeout", "proxyconnect",
	} {
		if strings.Contains(lower, marker) {
			return "infra"
		}
	}
	return "code"
}
