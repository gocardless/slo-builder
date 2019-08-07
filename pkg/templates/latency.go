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

// This map contains a mapping between request classes and the latency buckets
// we use in Prometheus. 
// 
// It assumes that the observation metric is an histogram where the following
// buckets exist: 0.1, 0.25, 0.5, 1, 2.5, 5 and 10.
//
// The bucket values were chosen based on the performance of payments-service
// over a  month. The spreadsheet linked below includes the latency percentiles
// of payments-service for each one of the routes over a month. We chose the
// bucket values below keeping in mind that:
// - We wanted to choose a set of buckets we could use across our 95th and 99th
//   latency percentiles
// - The buckets are generic and have an even distribution across all of routes,
//   i.e. all buckets apply to an similar number of routes so that there are no
//   special cases
//
// https://docs.google.com/spreadsheets/d/1CAe6gpgZdjYjBy44bdYSBD0ZA8yxSkMkiqjLiZ_8NRo/edit#gid=15427288
var requestClassToLatencyBucket = map[string]string{
  "fast++": "0.1", // 100 ms
  "fast+": "0.25", // 250 ms
  "fast": "0.5",   // 500 ms
  "ok": "1",       // 1s
  "slow": "2.5",   // 2.5s
  "slow+": "5",    // 5s
  "slow++": "10",  // 10s
}

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
			// TODO what if the class is invalid? Do we want to return an error?
			Expr:   fmt.Sprintf(l.Observation, requestClassToLatencyBucket[l.RequestClass], "%s"),
		}),
	)
}
