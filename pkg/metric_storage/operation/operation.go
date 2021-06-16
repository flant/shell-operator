package operation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/hashicorp/go-multierror"
)

type MetricOperation struct {
	Name string `json:"name"`
	// Deprecated: use Value + Action="add" instead. Add only works for parsing from file
	Add *float64 `json:"add,omitempty"` // shortcut for action=add value=num
	// Deprecated: use Value + Action="set" instead. Set only works for parsing from file
	Set     *float64          `json:"set,omitempty"` // shortcut for action=set value=num
	Value   *float64          `json:"value,omitempty"`
	Buckets []float64         `json:"buckets,omitempty"`
	Labels  map[string]string `json:"labels"`
	Group   string            `json:"group,omitempty"`
	Action  string            `json:"action,omitempty"`
}

func (m MetricOperation) String() string {
	parts := make([]string, 0)

	if m.Group != "" {
		parts = append(parts, "group="+m.Group)
	}
	if m.Name != "" {
		parts = append(parts, "name="+m.Name)
	}
	if m.Action != "" {
		parts = append(parts, "action="+m.Action)
	}

	if m.Value != nil {
		parts = append(parts, fmt.Sprintf("value=%f", *m.Value))
	}
	if m.Set != nil {
		parts = append(parts, fmt.Sprintf("set=%f", *m.Set))
	}
	if m.Add != nil {
		parts = append(parts, fmt.Sprintf("add=%f", *m.Add))
	}
	if m.Buckets != nil {
		parts = append(parts, fmt.Sprintf("buckets=%+v", m.Buckets))
	}
	if m.Labels != nil {
		parts = append(parts, fmt.Sprintf("labels=%+v", m.Labels))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

func MetricOperationsFromReader(r io.Reader) ([]MetricOperation, error) {
	var operations = make([]MetricOperation, 0)

	dec := json.NewDecoder(r)
	for {
		var metricOperation MetricOperation
		if err := dec.Decode(&metricOperation); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// shortcut transforms
		if metricOperation.Set != nil && metricOperation.Add == nil {
			metricOperation.Action = "set"
			metricOperation.Value = metricOperation.Set
		}
		if metricOperation.Add != nil && metricOperation.Set == nil {
			metricOperation.Action = "add"
			metricOperation.Value = metricOperation.Add
		}

		operations = append(operations, metricOperation)
	}

	return operations, nil
}

func MetricOperationsFromBytes(data []byte) ([]MetricOperation, error) {
	return MetricOperationsFromReader(bytes.NewReader(data))
}

func MetricOperationsFromFile(filePath string) ([]MetricOperation, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}
	return MetricOperationsFromBytes(data)
}

func ValidateOperations(ops []MetricOperation) error {
	var opsErrs *multierror.Error

	for _, op := range ops {
		err := ValidateMetricOperation(op)
		if err != nil {
			opsErrs = multierror.Append(opsErrs, err)
		}
	}

	return opsErrs.ErrorOrNil()
}

func ValidateMetricOperation(op MetricOperation) error {
	var opErrs *multierror.Error

	if op.Action == "" {
		opErrs = multierror.Append(opErrs, fmt.Errorf("one of: 'action', 'set' or 'add' is required: %s", op))
	}

	if op.Group == "" {
		if op.Action != "set" && op.Action != "add" && op.Action != "observe" {
			opErrs = multierror.Append(opErrs, fmt.Errorf("unsupported action '%s': %s", op.Action, op))
		}
	} else {
		if op.Action != "expire" && op.Action != "set" && op.Action != "add" {
			opErrs = multierror.Append(opErrs, fmt.Errorf("unsupported action '%s': %s", op.Action, op))
		}
	}

	if op.Name == "" && op.Group == "" {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required: %s", op))
	}
	if op.Name == "" && op.Group != "" && op.Action != "expire" {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'name' is required when action is not 'expire': %s", op))
	}

	if op.Action == "set" && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'set': %s", op))
	}
	if op.Action == "add" && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'add': %s", op))
	}
	if op.Action == "observe" && op.Value == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'value' is required for action 'observe': %s", op))
	}
	if op.Action == "observe" && op.Buckets == nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'buckets' is required for action 'observe': %s", op))
	}

	if op.Set != nil && op.Add != nil {
		opErrs = multierror.Append(opErrs, fmt.Errorf("'set' and 'add' are mutual exclusive: %s", op))
	}

	return opErrs.ErrorOrNil()
}
