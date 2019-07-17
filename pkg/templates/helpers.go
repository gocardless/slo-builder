package templates

import (
	"fmt"

	"github.com/prometheus/prometheus/pkg/rulefmt"
)

// flattenRules takes a list of either singleton Rule elements or slices, then flattens
// them into a single slice of Rules.
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

// forIntervals takes a Rule with string interpolation (%s) for any placeholders that
// should be templated with the given interval values.
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
