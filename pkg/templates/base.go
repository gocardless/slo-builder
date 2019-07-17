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

// BaseSLO is at the core of every SLO. Regardless of which template is used, every SLO
// must have an associated name and error budget. From this we produce two Prometheus
// rules:
//
// - job:slo_definition:none{name, budget, template_labels...}
// - job:slo_error_budget:ratio{name}
//
// The `slo_definition` is to track how the definition of an SLO changes over time. By
// recording the parameters provided to the template in a metric, it's easy to understand
// how other dependent recording rules were modified.
//
// The `slo_error_budget` rule is required to power generic alerting rules. Every template
// will eventually produce job:slo_error:ratio<I> rules, which together with the error
// budget can determine when to fire alerts.
type BaseSLO struct {
	Name   string  `json:"name"`
	Budget float64 `json:"budget"`
}

func (b BaseSLO) GetName() string {
	return b.Name
}

func (b BaseSLO) Rules(additionals ...map[string]string) []rulefmt.Rule {
	return []rulefmt.Rule{
		rulefmt.Rule{
			Record: "job:slo_definition:none",
			Labels: b.JoinLabels(
				append(additionals, map[string]string{
					"budget": fmt.Sprintf("%f", b.Budget),
				})...,
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

// JoinLabels allows templates to pass their additional labels into the definition rule
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
