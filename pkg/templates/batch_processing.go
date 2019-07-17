package templates

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

var (
	// BatchProcessingTemplateRules map from the job:slo_batch_* time series to the SLO
	// compliant job:slo_error:ratio<I> series that are used to power alerts.
	BatchProcessingTemplateRules = flattenRules(
		// Calculate synthentic 'error score' for the batch as the percentage of target
		// throughput we failed to achieve over the user defined interval.
		rulefmt.Rule{
			Record: "job:slo_batch_error:interval",
			Expr: `
1.0 - clamp_max(
  job:slo_batch_throughput:interval / job:slo_batch_throughput_target:max,
  1.0
)
			`,
		},
		// Use avg_over_time to map job:slo_batch_error:interval into error rate as measured
		// over the common alert window intervals.
		forIntervals(AlertWindows,
			rulefmt.Rule{
				Record: "job:slo_error:ratio%s",
				Expr:   `avg_over_time(job:slo_batch_error:interval[%s])`,
			},
		),
	)
)

func init() {
	MustRegisterTemplate(BatchProcessingSLO{}, BatchProcessingTemplateRules...)
}

// BatchProcessingSLO is used to construct SLOs around large batch processes that the
// business demands finishes within a given deadline.
//
// To use this template, you provide a measure of throughput for the batch process which
// is only present when the job is underway. The SLO then uses an estimated measure of
// maximum expected volume and the business deadline to compute a target throughput, then
// measures SLO compliance against how well the batch process meets the target.
//
// It can be a good idea to compute the volume measurement by taking a record of previous
// historic maximums and applying a growth multiplier that is appropriate for the business
// context. If you're processing a number of payments, and your peak volume comes once a
// month, expecting 1.5x the maximum volume processed by the batch job in the last 60 days
// might be a good starting point.
//
// The important characteristics of this SLO are:
//
//   - Error budget is consumed at a rate proportional to unmet target performance
//   - Error budget is consumed even by batches that process less-than-maximum volume
//
// One thing to note is that throughput exceeding the target threshold is considered 0%
// error, rather than some negative error value. This is a deliberate choice to avoid
// encouraging spiky throughput values, but may be toggled in future.
type BatchProcessingSLO struct {
	BaseSLO
	Deadline   Duration // time after starting the batch that it must finish
	Volume     string   // expected maximum volume to be processed by a single batch run
	Throughput string   // measure of batch throughput
}

func (b BatchProcessingSLO) Rules() []rulefmt.Rule {
	return append(
		b.BaseSLO.Rules(
			map[string]string{
				"deadline":   model.Duration(b.Deadline).String(),
				"volume":     b.Volume,
				"throughput": b.Throughput,
			},
		),
		rulefmt.Rule{
			Record: "job:slo_batch_volume:max",
			Labels: b.JoinLabels(),
			Expr:   b.Volume,
		},
		rulefmt.Rule{
			Record: "job:slo_batch_throughput_target:max",
			Labels: b.JoinLabels(),
			Expr: fmt.Sprintf(
				`job:slo_batch_volume:max / %d`, time.Duration(b.Deadline)/time.Second,
			),
		},
		rulefmt.Rule{
			Record: "job:slo_batch_throughput:interval",
			Labels: b.JoinLabels(),
			Expr:   b.Throughput,
		},
	)
}
