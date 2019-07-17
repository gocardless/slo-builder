package main

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"

	yaml "gopkg.in/yaml.v2"
)

// any slo: {
//   job:slo_error_budget:ratio
//   job:slo_definition:unit }

// batch, for each SLO: {
//	job:slo_batch_throughput_target:max
// 	job:slo_batch_throughput:interval }

// batch: {
//   job:slo_batch_error:interval
//   job:slo_error:ratio<I> }

// SLOErrorBudgetFastBurn
// SLOErrorBudgetSlowBurn

func MustRegister(rules ...rulefmt.Rule) {
	SLORules = append(SLORules, rules...)
}

var (
	// AlertWindows are common interval windows we want to precompute
	AlertWindows = []string{"1m", "5m", "30m", "1h", "6h", "1d", "3d", "7d"}

	// SLORules is where each SLO should place the appropriate rules that power the
	// post-processing and alert trailers.
	SLORules = []rulefmt.Rule{}

	// SLOBatchPreprocessing contains all generic recording rules that apply to each
	// batch-based SLO. We assume each batch SLO has provided slo_batch_throughput and
	// slo_batch_throughput_target.
	SLOBatchPostprocessing = flattenRules(
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

	// SLOAlerts every SLO type produces rules that terminate in job:slo_error:ratio<I> and
	// job:slo_error_budget's. Together, we can use these rules to power generic
	// multi-window SLO error budget burn alerts.
	SLOAlerts = []rulefmt.Rule{
		rulefmt.Rule{
			Alert: "SLOErrorBudgetFastBurn",
			For:   model.Duration(time.Minute),
			Labels: map[string]string{
				"severity": "ticket", // TODO: "page",
			},
			Expr: `
(
  job:slo_error:ratio1h > on(name) (14.4 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio5m > on(name) (14.4 * job:slo_error_budget:ratio)
)
or
(
  job:slo_error:ratio6h  > on(name) (6.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio30m > on(name) (6.0 * job:slo_error_budget:ratio)
)
			`,
		},
		rulefmt.Rule{
			Alert: "SLOErrorBudgetSlowBurn",
			For:   model.Duration(time.Hour),
			Labels: map[string]string{
				"severity": "ticket",
			},
			Expr: `
(
  job:slo_error:ratio1d > on(name) (3.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio2h > on(name) (3.0 * job:slo_error_budget:ratio)
)
or
(
  job:slo_error:ratio3d > on(name) (1.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio6h > on(name) (1.0 * job:slo_error_budget:ratio)
)
			`,
		},
	}
)

type BaseSLO struct {
	Name   string
	Budget float64
}

func (b BaseSLO) Rules() []rulefmt.Rule {
	return []rulefmt.Rule{
		rulefmt.Rule{
			Record: "job:slo_definition:none",
			Labels: b.JoinLabels(
				map[string]string{
					"budget": fmt.Sprintf("%f", b.Budget),
				},
			),
			Expr: "1",
		},
		rulefmt.Rule{
			Record: "job:slo_error_budget:ratio",
			Labels: b.JoinLabels(),
			Expr:   fmt.Sprintf("%f", b.Budget),
		},
	}
}

func (b BaseSLO) JoinLabels(additionals ...map[string]string) map[string]string {
	labels := map[string]string{
		"name": b.Name,
	}

	for _, additional := range additionals {
		for k, v := range additional {
			labels[k] = v
		}
	}

	return labels
}

func (b BaseSLO) Labels() map[string]string {
	return map[string]string{
		"name": b.Name,
	}
}

type BatchProcessingSLO struct {
	BaseSLO
	Deadline   time.Duration
	Volume     string
	Throughput string
}

func (b BatchProcessingSLO) Rules() []rulefmt.Rule {
	return append(
		b.BaseSLO.Rules(),
		rulefmt.Rule{
			Record: "job:slo_batch_volume:max",
			Labels: b.JoinLabels(),
			Expr:   b.Volume,
		},
		rulefmt.Rule{
			Record: "job:slo_batch_throughput_target:max",
			Labels: b.JoinLabels(),
			Expr: fmt.Sprintf(
				`job:slo_batch_volume:max / %d`, b.Deadline/time.Second,
			),
		},
		rulefmt.Rule{
			Record: "job:slo_batch_throughput:interval",
			Labels: b.JoinLabels(),
			Expr:   b.Throughput,
		},
	)
}

func main() {
	groups := rulefmt.RuleGroups{
		Groups: []rulefmt.RuleGroup{
			rulefmt.RuleGroup{
				Name: "slo-framework",
				Rules: flattenRules(
					SLORules,
					SLOBatchPostprocessing,
					SLOAlerts,
				),
			},
		},
	}

	groupsYaml, err := yaml.Marshal(groups)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(groupsYaml))
}

func flattenRules(elems ...interface{}) []rulefmt.Rule {
	flattened := []rulefmt.Rule{}
	for _, elem := range elems {
		if rule, ok := elem.(rulefmt.Rule); ok {
			flattened = append(flattened, rule)
		} else if rules, ok := elem.([]rulefmt.Rule); ok {
			flattened = append(flattened, rules...)
		} else {
			panic("unsupported type")
		}
	}

	return flattened
}

func forIntervals(intervals []string, rule rulefmt.Rule) []rulefmt.Rule {
	rules := []rulefmt.Rule{}
	for _, interval := range intervals {
		rules = append(
			rules,
			rulefmt.Rule{
				Record: fmt.Sprintf(rule.Record, interval),
				Expr:   fmt.Sprintf(rule.Expr, interval),
			},
		)
	}

	return rules
}
