package templates

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/rulefmt"
)

// SLO the base interface type for all SLOs
type SLO interface {
	// GetName returns a globally unique name for the SLO
	GetName() string
	// Rules generates Prometheus recording rules that implement the SLO definition
	Rules() []rulefmt.Rule
}

// baseSLO is at the core of every SLO. Regardless of which template is used, every SLO
// must have an associated name and error budget. From this we produce two Prometheus
// rules:
//
// - job:slo_definition:none{name, budget, template_labels...}
// - job:slo_error_budget:ratio{name}
// - job:slo_labels_info{name, definition_labels...}
//
// The `slo_definition` is to track how the definition of an SLO changes over time. By
// recording the parameters provided to the template in a metric, it's easy to understand
// how other dependent recording rules were modified.
//
// The `slo_error_budget` rule is required to power generic alerting rules. Every template
// will eventually produce job:slo_error:ratio<I> rules, which together with the error
// budget can determine when to fire alerts.
//
// The `slo_labels_info` provides additional labels that can be useful in the
// alerting rules.
//
type baseSLO struct {
	Name   string            `json:"name"`
	Budget float64           `json:"budget"`
	Labels map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func (b baseSLO) GetName() string {
	return b.Name
}

func (b baseSLO) Rules(additionals ...map[string]string) []rulefmt.Rule {
	return []rulefmt.Rule{
		rulefmt.Rule{
			Record: "job:slo_definition:none",
			Labels: b.joinLabels(
				append(additionals, map[string]string{
					"budget": fmt.Sprintf("%f", b.Budget),
				})...,
			),
			Annotations: b.Annotations,
			Expr: "1",
		},
		rulefmt.Rule{
			Record: "job:slo_error_budget:ratio",
			Labels: b.joinLabels(),
			Expr:   fmt.Sprintf("%f", b.Budget),
		},
		rulefmt.Rule{
			Record: "job:slo_labels_info",
			Labels: b.joinLabels(b.Labels),
			Expr:   "1",
		},
	}
}

// joinLabels allows templates to pass their additional labels into the definition rule
func (b baseSLO) joinLabels(additionals ...map[string]string) map[string]string {
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
