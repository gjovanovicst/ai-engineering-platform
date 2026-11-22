package domain

import "time"

// ReviewFindingStatus is the lifecycle of one review finding: a blocker stays
// open until a human dismisses it or a fix run closes it; a nit is the same
// with a lower weight.
type ReviewFindingStatus string

const (
	ReviewFindingOpen      ReviewFindingStatus = "open"
	ReviewFindingFixed     ReviewFindingStatus = "fixed"
	ReviewFindingDismissed ReviewFindingStatus = "dismissed"
)

// ReviewFindingSeverity weighs a finding: only a blocker gates the merge.
type ReviewFindingSeverity string

const (
	ReviewFindingBlocker ReviewFindingSeverity = "blocker"
	ReviewFindingNit     ReviewFindingSeverity = "nit"
)

// ReviewFinding is one finding of a review, with its lifecycle. The run_id is
// the review that last reported it.
type ReviewFinding struct {
	ID         string
	OrgID      string
	ProjectID  string
	TaskID     string
	RunID      string
	Severity   ReviewFindingSeverity
	File       string
	Line       int
	Message    string
	Status     ReviewFindingStatus
	DecidedBy  string
	Reason     string
	FixedByRun string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
