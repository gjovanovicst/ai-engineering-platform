// Package analytics computes delivery metrics from the run and approval log.
package analytics

import (
	"context"
	"errors"
	"log/slog"
)

// Store is the read port for metrics.
type Store interface {
	RunsByStatus(ctx context.Context, orgID string, days int) (map[string]int, error)
	RunDurationSeconds(ctx context.Context, orgID string, days int) (avg float64, count int, err error)
	ApprovalDecisionSeconds(ctx context.Context, orgID string, days int) (avg float64, count int, err error)
	AvgRunsPerTask(ctx context.Context, orgID string) (float64, error)
	MergedPullRequests(ctx context.Context, orgID string, days int) (int, error)
	UsageCost(ctx context.Context, orgID string, days int) (cost float64, rows int, err error)
	OnboardingSeconds(ctx context.Context, orgID string, days int) (avg float64, projects int, err error)
	FirstPassMergeRate(ctx context.Context, orgID string, days int) (float64, error)
	ProofValidity(ctx context.Context, orgID string, days int) (float64, int64, error)
	DevRunsPerTask(ctx context.Context, orgID string, days int) (float64, error)
	MergedPerDay(ctx context.Context, orgID string, days int) ([]DayPoint, error)
	LeadTimePerDay(ctx context.Context, orgID string, days int) ([]LeadTimePoint, error)
	Incidents(ctx context.Context, orgID string, days int) (int, error)
	MTTRSeconds(ctx context.Context, orgID string, days int) (float64, int, error)
	ProjectDORA(ctx context.Context, orgID string, days int) ([]ProjectDORA, error)
}

// Metric is a value with its provenance (ASDLC: never fake a number).
type Metric struct {
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Source string  `json:"source"` // measured | estimated | na
}

// DayPoint is one day of delivery activity.
type DayPoint struct {
	Date  string `json:"date"`
	Total int    `json:"total"`
}

// LeadTimePoint is average lead time for one day.
type LeadTimePoint struct {
	Date    string  `json:"date"`
	Minutes float64 `json:"minutes"`
}

// Regression is a metric that moved materially against its baseline.
type Regression struct {
	Metric       string  `json:"metric"`
	Current      float64 `json:"current"`
	Baseline     float64 `json:"baseline"`
	DeltaPercent float64 `json:"delta_percent"`
}

// ProjectDORA is per-project delivery health.
type ProjectDORA struct {
	ProjectID    string   `json:"project_id"`
	Name         string   `json:"name"`
	Merged       int      `json:"merged"`
	Incidents    int      `json:"incidents"`
	CFR          float64  `json:"change_failure_rate_percent"`
	MTTR         float64  `json:"mttr_minutes"`
	OnboardHours *float64 `json:"onboard_hours,omitempty"`
	TaskHours    *float64 `json:"task_hours,omitempty"`
	RunHours     *float64 `json:"run_hours,omitempty"`
	PRHours      *float64 `json:"pr_hours,omitempty"`
}

// Summary is the dashboard payload.
type Summary struct {
	Days               int             `json:"days"`
	RunsByStatus       map[string]int  `json:"runs_by_status"`
	SucceededRuns      int             `json:"succeeded_runs"`
	LeadTimeMinutes    Metric          `json:"lead_time_minutes"`
	ApprovalMinutes    Metric          `json:"approval_minutes"`
	RunsPerTask        Metric          `json:"runs_per_task"`
	MergedPullRequests int             `json:"merged_pull_requests"`
	Cost               Metric          `json:"cost_usd"`
	OnboardingHours    Metric          `json:"onboarding_hours"`
	SuccessRate        Metric          `json:"success_rate_percent"`
	CostPerAccepted    Metric          `json:"cost_per_accepted_change_usd"`
	DeployFrequency    Metric          `json:"deployments_per_day"`
	MergedTrend        []DayPoint      `json:"merged_trend"`
	LeadTimeTrend      []LeadTimePoint `json:"lead_time_trend"`
	Regressions        []Regression    `json:"regressions"`
	ChangeFailureRate  Metric          `json:"change_failure_rate_percent"`
	MTTRMinutes        Metric          `json:"mttr_minutes"`
	Projects           []ProjectDORA   `json:"projects"`
	FirstPassMerge     Metric          `json:"first_pass_merge_percent"`
	ProofValidity      Metric          `json:"proof_validity_percent"`
	DevRunsPerTask     Metric          `json:"dev_runs_per_task"`
	EscapedDefects     Metric          `json:"escaped_defects_per_100_merges"`
}

// Service computes summaries.
type Service struct {
	store Store
	log   *slog.Logger
}

// New wires the service.
func New(store Store, log *slog.Logger) (*Service, error) {
	if store == nil {
		return nil, errors.New("analytics: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, log: log}, nil
}

// Summary returns the metrics for the last days.
func (s *Service) Summary(ctx context.Context, orgID string, days int) (Summary, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	out := Summary{Days: days, RunsByStatus: map[string]int{}, MergedTrend: []DayPoint{}, LeadTimeTrend: []LeadTimePoint{}, Regressions: []Regression{}, Projects: []ProjectDORA{}}

	statuses, err := s.store.RunsByStatus(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.RunsByStatus = statuses
	out.SucceededRuns = statuses["succeeded"]

	avg, count, err := s.store.RunDurationSeconds(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.LeadTimeMinutes = measuredMinutes(avg, count)

	avgApproval, approvals, err := s.store.ApprovalDecisionSeconds(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.ApprovalMinutes = measuredMinutes(avgApproval, approvals)

	runsPerTask, err := s.store.AvgRunsPerTask(ctx, orgID)
	if err != nil {
		return Summary{}, err
	}
	out.RunsPerTask = Metric{Value: round(runsPerTask), Unit: "runs/task", Source: "measured"}

	merged, err := s.store.MergedPullRequests(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.MergedPullRequests = merged

	cost, costRows, err := s.store.UsageCost(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	if costRows > 0 {
		out.Cost = Metric{Value: round2(cost), Unit: "usd", Source: "measured"}
	} else {
		out.Cost = Metric{Value: 0, Unit: "usd", Source: "na"}
	}
	total := 0
	for _, n := range statuses {
		total += n
	}
	if total > 0 {
		out.SuccessRate = Metric{Value: round2(100 * float64(statuses["succeeded"]) / float64(total)), Unit: "percent", Source: "measured"}
	} else {
		out.SuccessRate = Metric{Unit: "percent", Source: "na"}
	}
	if out.MergedPullRequests > 0 && costRows > 0 {
		out.CostPerAccepted = Metric{Value: round2(cost / float64(out.MergedPullRequests)), Unit: "usd", Source: "measured"}
	} else {
		out.CostPerAccepted = Metric{Unit: "usd", Source: "na"}
	}
	out.DeployFrequency = Metric{Value: round2(float64(out.MergedPullRequests) / float64(days)), Unit: "per_day", Source: "measured"}
	onboarding, onboarded, err := s.store.OnboardingSeconds(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.OnboardingHours = onboardingMetric(onboarding, onboarded)
	firstPass, err := s.store.FirstPassMergeRate(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	if out.MergedPullRequests > 0 {
		out.FirstPassMerge = Metric{Value: round2(100 * firstPass), Unit: "percent", Source: "measured"}
	} else {
		out.FirstPassMerge = Metric{Unit: "percent", Source: "na"}
	}
	proofShare, proofRuns, err := s.store.ProofValidity(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	if proofRuns > 0 {
		out.ProofValidity = Metric{Value: round2(100 * proofShare), Unit: "percent", Source: "measured"}
	} else {
		out.ProofValidity = Metric{Unit: "percent", Source: "na"}
	}
	devRuns, err := s.store.DevRunsPerTask(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.DevRunsPerTask = Metric{Value: round2(devRuns), Unit: "runs", Source: "measured"}
	out.MergedTrend, err = s.store.MergedPerDay(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.LeadTimeTrend, err = s.store.LeadTimePerDay(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	if regression, ok := leadTimeRegression(out.LeadTimeTrend); ok {
		out.Regressions = append(out.Regressions, regression)
	}
	incidentCount, err := s.store.Incidents(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	if out.MergedPullRequests > 0 {
		out.ChangeFailureRate = Metric{Value: round2(100 * float64(incidentCount) / float64(out.MergedPullRequests)), Unit: "percent", Source: "measured"}
		out.EscapedDefects = Metric{Value: round2(100 * float64(incidentCount) / float64(out.MergedPullRequests)), Unit: "per_100_merges", Source: "measured"}
	} else {
		out.ChangeFailureRate = Metric{Unit: "percent", Source: "na"}
		out.EscapedDefects = Metric{Unit: "per_100_merges", Source: "na"}
	}
	mttr, resolved, err := s.store.MTTRSeconds(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	out.Projects, err = s.store.ProjectDORA(ctx, orgID, days)
	if err != nil {
		return Summary{}, err
	}
	for i := range out.Projects {
		if out.Projects[i].Merged > 0 {
			out.Projects[i].CFR = round2(100 * float64(out.Projects[i].Incidents) / float64(out.Projects[i].Merged))
		}
		out.Projects[i].MTTR = round2(out.Projects[i].MTTR / 60)
	}
	if resolved > 0 {
		out.MTTRMinutes = Metric{Value: round2(mttr / 60), Unit: "minutes", Source: "measured"}
	} else {
		out.MTTRMinutes = Metric{Unit: "minutes", Source: "na"}
	}
	return out, nil
}

func measuredMinutes(seconds float64, count int) Metric {
	if count == 0 {
		return Metric{Value: 0, Unit: "minutes", Source: "na"}
	}
	return Metric{Value: round(seconds / 60), Unit: "minutes", Source: "measured"}
}

func round2(v float64) float64 {
	if v < 0 {
		return 0
	}
	return float64(int(v*100+0.5)) / 100
}

func round(v float64) float64 {
	if v < 0 {
		return 0
	}
	return float64(int(v*10+0.5)) / 10
}

func onboardingMetric(avgSeconds float64, projects int) Metric {
	if projects == 0 {
		return Metric{Unit: "hours", Source: "na"}
	}
	return Metric{Value: round2(avgSeconds / 3600), Unit: "hours", Source: "measured"}
}
