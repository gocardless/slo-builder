package templates

import (
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

var (
	// ErrorRateTemplateRules map from the job:slo_error_rate_total and
	// job:slo_error_rate_errors time series to the SLO-compliant
	// job:slo_error:ratio<I> series that are used to power alerts.
	ErrorRateTemplateRules = flattenRules(
		// Calculate error rate ratio
		// Worth noting that job:slo_error_rate_errors could be NaN so we
		// need to ensure that it's 0 or a scalar
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_error:ratio%s",
			Expr:   `((job:slo_error_rate_errors:rate%[1]s) or (0 * job:slo_error_rate_total:rate%[1]s)) / job:slo_error_rate_total:rate%[1]s`,
		}),
	)
)

func init() {
	MustRegisterTemplate(ErrorRateSLO{}, ErrorRateTemplateRules...)
}

// ErrorRateSLO is used to construct SLOs based on error rate.

// To use this template, you provide a parameterised rate of requests and
// errors that are sliced across multiple time windows.
type ErrorRateSLO struct {
	baseSLO
	Errors string
	Total  string
}

func (e ErrorRateSLO) Rules() []rulefmt.Rule {
	return flattenRules(
		e.baseSLO.Rules(
			map[string]string{
				"template": "ErrorRateSLO",
				"errors": e.Errors,
				"total":  e.Total,
			},
		),
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_error_rate_errors:rate%s",
			Labels: e.joinLabels(),
			Expr:   e.Errors,
		}),
		forIntervals(AlertWindows, rulefmt.Rule{
			Record: "job:slo_error_rate_total:rate%s",
			Labels: e.joinLabels(),
			Expr:   e.Total,
		}),
	)
}
