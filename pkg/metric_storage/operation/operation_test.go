package operation

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_ValidateMetricOperations(t *testing.T) {
	//g := NewWithT(t)
	tests := []struct {
		name     string
		op       string
		expected bool
	}{
		{
			"simple",
			`{"group":"someGroup", "action":"expire"}`,
			true,
		},
		{
			"action set",
			`{"name":"metric_1", "action":"set", "value":1}`,
			true,
		},
		{
			"set shortcut",
			`{"name":"metric_1", "set":1}`,
			true,
		},
		{
			"invalid",
			`{"group":"someGroup", "action":"expired"}`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ops, err := MetricOperationsFromBytes([]byte(tt.op))
			g.Expect(err).ShouldNot(HaveOccurred())

			err = ValidateOperations(ops)
			if tt.expected {
				g.Expect(err).ShouldNot(HaveOccurred())
			} else {
				//t.Logf("expected error: %v", err)
				g.Expect(err).Should(HaveOccurred())
			}
		})
	}
}
