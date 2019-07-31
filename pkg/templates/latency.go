package templates

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/rulefmt"
)

var (
	// LatencyTemplateRules map from the job:slo_latency_* time series to the
	// SLO-compliant job:slo_error:ratio<I> series than are used to power
	// alerts.
	LatencyTemplateRules = flattenRules(
		// Calculate the ratio of requests above the observation, divided by
		// the total requests.
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_error:ratio%s",
			Expr:   `(job:slo_latency_total:rate%[1]s - job:slo_latency_observation:rate%[1]s) / job:slo_latency_total:rate%[1]s`,
		}),
	)
)

func init() {
	MustRegisterTemplate(LatencySLO{}, LatencyTemplateRules...)
}

// LatencySLO is used to construct SLOs based on latency.
//
// To use this template, you provide a parameterized rate of total requests, a
// parameterized counter that tracks the number of observations (histogram
// bucket) and request class that references a latency target.
//
// This template allows defining SLOs as follows:
//
// 90% requests < 300ms
// 99% requests < 1000ms
//
type LatencySLO struct {
	baseSLO
	RequestClass string // request class references a latency target
	Total        string // parameterized rate of total requests
	Observation  string // parameterized rate of histogram bucket
}

func (l LatencySLO) Rules() []rulefmt.Rule {
	return flattenRules(
		l.baseSLO.Rules(
			map[string]string{
				"request_class": l.RequestClass,
				"total":         l.Total,
				"observation":   l.Observation,
			},
		),
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_latency_total:rate%s",
			Labels: l.joinLabels(map[string]string{"request_class": l.RequestClass}),
			Expr:   l.Total,
		}),
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_latency_observation:rate%s",
			Labels: l.joinLabels(map[string]string{"request_class": l.RequestClass}),
			Expr:   fmt.Sprintf(l.Observation, l.RequestClass, "%s"),
		}),
	)
}
