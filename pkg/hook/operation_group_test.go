package hook

import (
	"testing"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/onsi/gomega"
)

func TestMetricOperationsFromBytes_CounterAddWithoutDefaultGroup(t *testing.T) {
	g := NewWithT(t)

	ops, err := MetricOperationsFromBytes([]byte(`{"name":"my_counter","action":"add","value":1}`), "my-hook")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ops).To(HaveLen(1))
	g.Expect(ops[0].Group).To(BeEmpty())
}

func TestMetricOperationsFromBytes_GaugeSetGetsDefaultGroup(t *testing.T) {
	g := NewWithT(t)

	ops, err := MetricOperationsFromBytes([]byte(`{"name":"my_gauge","action":"set","value":1}`), "my-hook")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ops).To(HaveLen(1))
	g.Expect(ops[0].Group).To(Equal("my-hook"))
}

func TestMetricOperationsFromBytes_ExplicitGroupPreservedForCounter(t *testing.T) {
	g := NewWithT(t)

	ops, err := MetricOperationsFromBytes([]byte(`{"name":"my_counter","action":"add","value":1,"group":"custom"}`), "my-hook")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ops[0].Group).To(Equal("custom"))
}

func TestCounterAddAccumulatesAcrossGroupedBatches(t *testing.T) {
	g := NewWithT(t)

	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())

	remap := func(ops []MetricOperation) []operation.MetricOperation {
		result := make([]operation.MetricOperation, 0, len(ops))
		for _, op := range ops {
			val := *op.Value
			result = append(result, operation.MetricOperation{
				Name:   op.Name,
				Value:  &val,
				Group:  op.Group,
				Action: operation.ActionCounterAdd,
			})
		}
		return result
	}

	for i := 0; i < 3; i++ {
		parsed, err := MetricOperationsFromBytes([]byte(`{"name":"runs_total","action":"add","value":1}`), "cron-hook")
		g.Expect(err).NotTo(HaveOccurred())
		err = ms.ApplyBatchOperations(remap(parsed), nil)
		g.Expect(err).NotTo(HaveOccurred())
	}

	families, err := ms.Gather()
	g.Expect(err).NotTo(HaveOccurred())

	var value float64
	for _, family := range families {
		if family.GetName() == "runs_total" {
			value = family.GetMetric()[0].GetCounter().GetValue()
			break
		}
	}
	g.Expect(value).To(Equal(float64(3)))
}

func TestCounterAddWithExplicitGroupStillExpires(t *testing.T) {
	g := NewWithT(t)

	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())

	op := operation.MetricOperation{
		Name:   "grouped_counter",
		Group:  "my-group",
		Action: operation.ActionCounterAdd,
	}
	val := 5.0
	op.Value = &val

	err := ms.ApplyBatchOperations([]operation.MetricOperation{op}, nil)
	g.Expect(err).NotTo(HaveOccurred())

	err = ms.ApplyBatchOperations([]operation.MetricOperation{op}, nil)
	g.Expect(err).NotTo(HaveOccurred())

	families, err := ms.Gather()
	g.Expect(err).NotTo(HaveOccurred())

	var value float64
	for _, family := range families {
		if family.GetName() == "grouped_counter" {
			value = family.GetMetric()[0].GetCounter().GetValue()
			break
		}
	}
	// Grouped path expires before apply, so the counter is reset each batch.
	g.Expect(value).To(Equal(float64(5)))
}
