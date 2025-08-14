package operation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
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

func MetricOperationsFromReader(r io.Reader, defaultGroup string) ([]operation.MetricOperation, error) {
	operations := make([]operation.MetricOperation, 0)

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

		if metricOperation.Group == "" {
			metricOperation.Group = defaultGroup
		}

		op := operation.MetricOperation{
			Name:    metricOperation.Name,
			Value:   metricOperation.Value,
			Buckets: metricOperation.Buckets,
			Labels:  metricOperation.Labels,
			Group:   metricOperation.Group,
		}

		operations = append(operations, op)
	}

	return operations, nil
}

func remapMetricOperations(ops []MetricOperation) []operation.MetricOperation {
	newOps := make([]operation.MetricOperation, len(ops))
	for _, op := range ops {
		newOp := operation.MetricOperation{
			Name:    op.Name,
			Value:   op.Value,
			Buckets: op.Buckets,
			Labels:  op.Labels,
			Group:   op.Group,
		}

		switch op.Action {
		case "add":
			newOp.Action = operation.ActionCounterAdd
		case "set":
			newOp.Action = operation.ActionGaugeSet
		case "observe":
			newOp.Action = operation.ActionHistogramObserve
		case "expire":
			newOp.Action = operation.ActionExpireMetrics
		}

		newOps = append(newOps, newOp)
	}

	return newOps
}

func MetricOperationsFromBytes(data []byte, defaultGroup string) ([]operation.MetricOperation, error) {
	return MetricOperationsFromReader(bytes.NewReader(data), defaultGroup)
}

func MetricOperationsFromFile(filePath, defaultGroup string) ([]operation.MetricOperation, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", filePath, err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	return MetricOperationsFromBytes(data, defaultGroup)
}
