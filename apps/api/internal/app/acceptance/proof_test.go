package acceptance

import (
	"strings"
	"testing"
)

func TestParseAndValidateProof(t *testing.T) {
	lines := []string{"done", `AIP_PROOF: {"evidence":[{"criterion":"AC1","kind":"automated","test":"TestPager","files":["pagination_test.go"]}]}`}
	proof, err := ParseProof(lines)
	if err != nil {
		t.Fatal(err)
	}
	diff := "+++ b/pagination_test.go\n+func TestPager(t *testing.T) {}\n+func TestNext(t *testing.T) {}\n"
	res := ValidateProof(proof, []string{"AC1"}, diff)
	if !res.Passed || res.Automated != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestProofRequiresEvidenceForEveryCriterion(t *testing.T) {
	proof := &Proof{Evidence: []Evidence{{Criterion: "AC1", Kind: "manual", Reason: "reviewed by QA"}}}
	res := ValidateProof(proof, []string{"AC1", "AC2"}, "+++ b/main.go\n+x\n")
	if res.Passed || !strings.Contains(res.Detail, "no evidence") {
		t.Fatalf("result = %+v", res)
	}
}

func TestProofRejectsTestNotInDiff(t *testing.T) {
	proof := &Proof{Evidence: []Evidence{{Criterion: "AC1", Kind: "automated", Test: "TestPager", Files: []string{"pagination_test.go"}}}}
	res := ValidateProof(proof, []string{"AC1"}, "+++ b/main.go\n+changed\n")
	if res.Passed {
		t.Fatal("expected failure for a test that is not in the diff")
	}
}

func TestParseProofMissing(t *testing.T) {
	if _, err := ParseProof([]string{"no proof"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTestFilesDeduplicates(t *testing.T) {
	proof := &Proof{Evidence: []Evidence{
		{Criterion: "AC1", Kind: "automated", Files: []string{"a_test.go"}},
		{Criterion: "AC2", Kind: "automated", Files: []string{"a_test.go", "b_test.go"}},
		{Criterion: "AC3", Kind: "manual", Reason: "reviewed"},
	}}
	files := TestFiles(proof)
	if len(files) != 2 {
		t.Fatalf("files = %v", files)
	}
}
