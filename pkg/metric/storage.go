package metric

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
)

type Storage interface {
	ApplyOperation(op operation.MetricOperation, commonLabels map[string]string)

	Counter(metric string, labels map[string]string) *prometheus.CounterVec
	CounterAdd(metric string, value float64, labels map[string]string)

	Gauge(metric string, labels map[string]string) *prometheus.GaugeVec
	GaugeAdd(metric string, value float64, labels map[string]string)
	GaugeSet(metric string, value float64, labels map[string]string)

	Grouped() GroupedStorage

	Handler() http.Handler

	Histogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)

	RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec
	RegisterGauge(metric string, labels map[string]string) *prometheus.GaugeVec
	RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec

	SendBatch(ops []operation.MetricOperation, labels map[string]string) error

	Cleanup()
}

type GroupedStorage interface {
	Registerer() prometheus.Registerer
	ExpireGroupMetrics(group string)
	ExpireGroupMetricByName(group, name string)
	GetOrCreateCounterCollector(name string, labelNames []string) (*ConstCounterCollector, error)
	GetOrCreateGaugeCollector(name string, labelNames []string) (*ConstGaugeCollector, error)
	CounterAdd(group string, name string, value float64, labels map[string]string)
	GaugeSet(group string, name string, value float64, labels map[string]string)
}
