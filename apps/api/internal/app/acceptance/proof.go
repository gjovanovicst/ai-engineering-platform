package acceptance

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Evidence is one proof entry for an acceptance criterion.
type Evidence struct {
	Criterion string   `json:"criterion"`
	Kind      string   `json:"kind"` // automated | manual
	Test      string   `json:"test,omitempty"`
	Files     []string `json:"files,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

// Proof is the structured evidence emitted by the development agent.
type Proof struct {
	Evidence []Evidence `json:"evidence"`
}

// Result is the validated outcome.
type Result struct {
	Passed    bool
	Detail    string
	Automated int
	Manual    int
	Proof     Proof
}

const proofPrefix = "AIP_PROOF:"

// ParseProof finds the last AIP_PROOF line.
func ParseProof(lines []string) (*Proof, error) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, proofPrefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, proofPrefix))
		var proof Proof
		if err := json.Unmarshal([]byte(raw), &proof); err != nil {
			return nil, fmt.Errorf("AIP_PROOF is not valid JSON: %w", err)
		}
		return &proof, nil
	}
	return nil, fmt.Errorf("the development agent did not emit an AIP_PROOF line")
}

// ValidateProof checks that every criterion has acceptable evidence and that
// automated tests are present in the diff.
func ValidateProof(proof *Proof, criteria []string, diff string) Result {
	byCriterion := map[string][]Evidence{}
	for _, item := range proof.Evidence {
		byCriterion[item.Criterion] = append(byCriterion[item.Criterion], item)
	}
	added := addedLines(diff)
	files := changedFiles(diff)
	var missing, invalid, manual, automated int
	for _, criterion := range criteria {
		items := byCriterion[criterion]
		if len(items) == 0 {
			missing++
			continue
		}
		valid := false
		for _, item := range items {
			switch item.Kind {
			case "manual":
				if len(strings.TrimSpace(item.Reason)) >= 5 {
					manual++
					valid = true
				}
			case "automated":
				if item.Test == "" {
					continue
				}
				if !testPresent(item.Test, added) {
					continue
				}
				if len(item.Files) > 0 && !anyChanged(item.Files, files) {
					continue
				}
				automated++
				valid = true
			}
			if valid {
				break
			}
		}
		if !valid {
			invalid++
		}
	}
	res := Result{Automated: automated, Manual: manual, Proof: *proof}
	switch {
	case missing > 0:
		res.Detail = fmt.Sprintf("%d acceptance criterion/criteria have no evidence", missing)
	case invalid > 0:
		res.Detail = fmt.Sprintf("%d criterion/criteria have evidence that is not verifiable (test not in the diff, missing file, or manual without a reason)", invalid)
	default:
		res.Passed = true
		res.Detail = fmt.Sprintf("%d automated and %d manual evidence entries", automated, manual)
	}
	return res
}

func addedLines(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func changedFiles(diff string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			out[strings.TrimPrefix(line, "+++ b/")] = true
		}
	}
	return out
}

func testPresent(name, added string) bool {
	return strings.Contains(added, name)
}

func anyChanged(want []string, files map[string]bool) bool {
	for _, file := range want {
		if files[file] {
			return true
		}
	}
	return false
}

// TempTests lists temporary test files in the diff. They are a warning, not a
// failure: the agent may have left one behind while investigating.
func TempTests(diff string) []string {
	var out []string
	for file := range changedFiles(diff) {
		base := file
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		if !strings.HasSuffix(base, "_test.go") {
			continue
		}
		if strings.HasPrefix(base, "tmp_") || strings.HasPrefix(base, "zz_") {
			out = append(out, file)
		}
	}
	return out
}

// TestFiles returns the unique test files referenced by automated evidence.
func TestFiles(proof *Proof) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range proof.Evidence {
		if item.Kind != "automated" {
			continue
		}
		for _, file := range item.Files {
			if file != "" && !seen[file] {
				seen[file] = true
				out = append(out, file)
			}
		}
	}
	return out
}
