package analytics

import "sort"

// leadTimeRegression compares the latest day with the median of prior days.
// It requires at least three points and flags a >50% rise of at least 10 minutes.
func leadTimeRegression(points []LeadTimePoint) (Regression, bool) {
	if len(points) < 3 {
		return Regression{}, false
	}
	latest := points[len(points)-1]
	prior := make([]float64, 0, len(points)-1)
	for _, point := range points[:len(points)-1] {
		prior = append(prior, point.Minutes)
	}
	sort.Float64s(prior)
	baseline := prior[len(prior)/2]
	if latest.Minutes < baseline*1.5 || latest.Minutes-baseline < 10 {
		return Regression{}, false
	}
	return Regression{
		Metric:       "lead_time_minutes",
		Current:      latest.Minutes,
		Baseline:     baseline,
		DeltaPercent: (latest.Minutes - baseline) / baseline * 100,
	}, true
}
