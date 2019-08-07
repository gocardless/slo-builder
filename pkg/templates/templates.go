// This package contains the implementation of various SLO templates.
//
// When creating SLOs for services, once the key SLIs have been defined it's necessary to
// forumulate an expression that can represent how well the service is performing in
// accordance to its objectives.
//
// Finding the expression that will take into account many key performance objectives of
// the system while being understandable can be difficult. The value of this package is to
// provide pre-configured templates that map to different categories of system that cater
// for the common properties people care about, enabling developers to apply sensible SLOs
// without having to dive deep into SLO-theory and Prometheus details.
//
// Each template registers itself with a global registry, at which point it's possible to
// use the template in a definition file provided to the build command. Pipelines then
// construct a rule group in the order required to power each different template, while
// feeding into a common set of alerting windows that apply to all SLOs.
package templates

import (
	"reflect"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

// MustRegisterTemplate installs the rules that map template specific SLO intermediate
// calculations to the job:slo_error:ratio<I> series that power alerts. This is called
// from the place a template is implemented.
func MustRegisterTemplate(slo SLO, rules ...rulefmt.Rule) {
	Templates[reflect.TypeOf(slo).Name()] = slo
	TemplateRules = append(TemplateRules, rules...)
}

var (
	// Templates stores a mapping of template name to registered template. This is used to
	// unmarshal template definitions from their yaml source and to provide users with
	// feedback about what templates this tool supports.
	Templates = map[string]SLO{}

	// TemplateRules implement the translation from the rules produced by each instance of
	// SLO templates into the generic SLO error:ratio<I> format, which then power alerts.
	TemplateRules = []rulefmt.Rule{}

	// AlertWindows are common interval windows we want to precompute
	AlertWindows = []string{"1m", "5m", "30m", "1h", "2h", "6h", "1d", "3d", "7d", "28d"}

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
  job:slo_error:ratio1h > on(name) group_left() (14.4 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio5m > on(name) group_left() (14.4 * job:slo_error_budget:ratio)
)
or
(
  job:slo_error:ratio6h > on(name) group_left() (6.0 * job:slo_error_budget:ratio)
and
  job:slo_error:ratio30m > on(name) group_left() (6.0 * job:slo_error_budget:ratio)
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
