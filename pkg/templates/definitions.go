package templates

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/ghodss/yaml"
	"github.com/prometheus/common/model"
)

// Duration supports unmarshaling from JSON using the same logic Prometheus uses to
// interpret human durations.
type Duration time.Duration

func (d *Duration) UnmarshalJSON(payload []byte) error {
	var human string
	if err := json.Unmarshal(payload, &human); err != nil {
		return err
	}

	parsed, err := model.ParseDuration(human)
	*d = Duration(parsed)
	return err
}

// ParseDefinitions loads a YAML file of configured templates that looks like this:
//
// ---
// definitions:
//   - template: BatchProcessingSLO
//     definition:
//       name: MarkPaymentsAsPaidMeetsDeadline
//       ...
//
// and produces a list of SLOs. This is the file format we expect users to be providing to
// the slo-builder.
func ParseDefinitions(payload []byte) ([]SLO, error) {
	envelope := struct {
		Definitions []SLOEnvelope `json:"definitions"`
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
type SLOEnvelope struct {
	SLO
}

func (s *SLOEnvelope) UnmarshalJSON(payload []byte) error {
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
