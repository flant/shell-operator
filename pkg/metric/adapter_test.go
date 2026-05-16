package metric_test

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	dto "github.com/prometheus/client_model/go"
	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/metric"
)

func TestMetricsAdapter_ReplacesPrefixInMetricNames(t *testing.T) {
	g := NewWithT(t)

	const prefix = "shell_operator_"
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	adapter := metric.NewMetricsAdapter(ms, prefix, log.NewNop())

	metricName := "{PREFIX}kubernetes_client_rate_limiter_latency_seconds"
	adapter.RegisterHistogram(metricName, map[string]string{"verb": "GET"}, []float64{0.005, 0.01})
	adapter.HistogramObserve(metricName, 0.001, map[string]string{"verb": "GET"}, []float64{0.005, 0.01})

	expectedName := prefix + "kubernetes_client_rate_limiter_latency_seconds"
	families, err := ms.Gather()
	g.Expect(err).NotTo(HaveOccurred())

	var found bool
	for _, family := range families {
		if family.GetName() == expectedName {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "expected metric %q in gather output, got %v", expectedName, metricFamilyNames(families))
}

func TestMetricsAdapter_ReplacesPrefixForCounter(t *testing.T) {
	g := NewWithT(t)

	const prefix = "test_"
	ms := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	adapter := metric.NewMetricsAdapter(ms, prefix, log.NewNop())

	metricName := "{PREFIX}example_total"
	adapter.RegisterCounter(metricName, nil)
	adapter.CounterAdd(metricName, 1, nil)

	expectedName := prefix + "example_total"
	families, err := ms.Gather()
	g.Expect(err).NotTo(HaveOccurred())

	var found bool
	for _, family := range families {
		if family.GetName() == expectedName {
			found = true
			g.Expect(family.GetMetric()[0].GetCounter().GetValue()).To(Equal(float64(1)))
			break
		}
	}
	g.Expect(found).To(BeTrue())
}

func metricFamilyNames(families []*dto.MetricFamily) []string {
	names := make([]string, 0, len(families))
	for _, f := range families {
		names = append(names, f.GetName())
	}
	return names
}
