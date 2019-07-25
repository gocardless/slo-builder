package templates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/ghodss/yaml"
	"github.com/prometheus/common/model"
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

// forIntervals takes a Rule with string interpolation (%[1]s) for any placeholders that
// should be templated with the given interval values.
func forIntervals(intervals []string, rule rulefmt.Rule) []rulefmt.Rule {
	rules := []rulefmt.Rule{}
	for _, interval := range intervals {
		rules = append(
			rules,
			rulefmt.Rule{
				Record: fmt.Sprintf(rule.Record, interval),
				Expr:   fmt.Sprintf(rule.Expr, interval),
				Labels: rule.Labels,
			},
		)
	}

	return rules
}

// serializableDuration supports unmarshaling from JSON using the same logic Prometheus
// uses to interpret human durations.
type serializeableDuration time.Duration

func (d *serializeableDuration) UnmarshalJSON(payload []byte) error {
	var human string
	if err := json.Unmarshal(payload, &human); err != nil {
		return err
	}

	parsed, err := model.ParseDuration(human)
	*d = serializeableDuration(parsed)
	return err
}

// ParseDefinitions loads a YAML file of configured templates that looks like this:
//
//   ---
//   definitions:
//     - template: BatchProcessingSLO
//       definition:
//         name: MarkPaymentsAsPaidMeetsDeadline
//         ...
//
// and produces a list of SLOs. This is the file format we expect users to be providing to
// the slo-builder.
func ParseDefinitions(payload []byte) ([]SLO, error) {
	envelope := struct {
		Definitions []sloEnvelope `json:"definitions"`
	}{}

	if err := yaml.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}

	slos := []SLO{}
	for _, sloEnvelope := range envelope.Definitions {
		slos = append(slos, sloEnvelope.SLO)
	}

	return slos, nil
}

// SLOEnvelope provides unmarshaling logic to parse a configured SLO template type from
// the definition schema. It can only parse templates that have been registered, and has
// to do a bit of reflection to dynamically support each type.
type sloEnvelope struct {
	SLO
}

func (s *sloEnvelope) UnmarshalJSON(payload []byte) error {
	envelope := struct {
		Template   string          `json:"template"`
		Definition json.RawMessage `json:"definition"`
	}{}

	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}

	tpl, ok := Templates[envelope.Template]
	if !ok {
		return fmt.Errorf("unsupported template type: %s", envelope.Template)
	}

	// Initialise a new SLO from the registered concrete type
	s.SLO = reflect.New(reflect.TypeOf(tpl)).Interface().(SLO)

	return json.Unmarshal(envelope.Definition, s.SLO)
}
