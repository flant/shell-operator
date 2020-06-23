package operation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

type MetricOperation struct {
	Name   string            `json:"name"`
	Add    *float64          `json:"add,omitempty"`
	Set    *float64          `json:"set,omitempty"`
	Labels map[string]string `json:"labels"`
	Group  string            `json:"group,omitempty"`
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
