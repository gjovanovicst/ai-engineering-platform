package analytics

import "testing"

func TestLeadTimeRegression(t *testing.T) {
	points := []LeadTimePoint{{Date: "a", Minutes: 10}, {Date: "b", Minutes: 12}, {Date: "c", Minutes: 40}}
	regression, ok := leadTimeRegression(points)
	if !ok || regression.Metric != "lead_time_minutes" || regression.DeltaPercent <= 0 {
		t.Fatalf("regression=%+v ok=%v", regression, ok)
	}
	if _, ok := leadTimeRegression([]LeadTimePoint{{Date: "a", Minutes: 10}, {Date: "b", Minutes: 11}}); ok {
		t.Fatal("two points must not flag")
	}
	if _, ok := leadTimeRegression([]LeadTimePoint{{Date: "a", Minutes: 10}, {Date: "b", Minutes: 11}, {Date: "c", Minutes: 12}}); ok {
		t.Fatal("flat trend must not flag")
	}
}
