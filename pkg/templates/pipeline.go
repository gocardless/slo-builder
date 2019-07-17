package templates

import (
	"reflect"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

// Pipeline can build a RuleGroup that powers the generation of SLO time series. When
// defining a template, you should add
type Pipeline struct {
	// Name defines the RuleGroup name in Prometheus
	Name string

	// SLORules is where each SLO should place the appropriate rules that power the
	// post-processing and alert trailers.
	SLORules []rulefmt.Rule
}

func NewPipeline(name string) *Pipeline {
	return &Pipeline{name, []rulefmt.Rule{}}
}

func (p *Pipeline) MustRegister(slos ...SLO) {
	for _, slo := range slos {
		p.SLORules = append(p.SLORules, slo.Rules()...)
	}
}

func (p *Pipeline) Build() rulefmt.RuleGroups {
	return rulefmt.RuleGroups{
		Groups: []rulefmt.RuleGroup{
			rulefmt.RuleGroup{
				Name: p.Name,
				Rules: flattenRules(
					p.SLORules,
					TemplateRules,
					AlertRules,
				),
			},
		},
	}
}

// MustRegisterTemplate installs the rules that map template specific SLO intermediate
// calculations to the job:slo_error:ratio<I> series that power alerts. This is called
// from the place a template is implemented.
func MustRegisterTemplate(slo SLO, rules ...rulefmt.Rule) {
	Templates[reflect.TypeOf(slo).Name()] = slo
	TemplateRules = append(TemplateRules, rules...)
}

var (
	// Templates stores a mapping of template name to registered template
	Templates = map[string]SLO{}

	// TemplateRules implement the translation from the rules produced by each instance of
	// SLO templates into the generic SLO error:ratio<I> format, which then power alerts.
	TemplateRules = []rulefmt.Rule{}

	// AlertWindows are common interval windows we want to precompute
	AlertWindows = []string{"1m", "5m", "30m", "1h", "6h", "1d", "3d", "7d", "28d"}

	// AlertRules every SLO type produces rules that terminate in job:slo_error:ratio<I> and
	// job:slo_error_budget's. Together, we can use these rules to power generic
	// multi-window SLO error budget burn alerts, and these alert rules are run as the final
	// part of the Pipeline generated RuleGroup.
	AlertRules = []rulefmt.Rule{
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
  job:slo_error:ratio1d > on(name) group_left() (3.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio2h > on(name) group_left() (3.0 * job:slo_error_budget:ratio)
)
or
(
  job:slo_error:ratio3d > on(name) group_left() (1.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio6h > on(name) group_left() (1.0 * job:slo_error_budget:ratio)
)
			`,
		},
	}
)
