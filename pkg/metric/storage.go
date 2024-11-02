package metric

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
)

type Storage interface {
	GaugeSet(metric string, value float64, labels map[string]string)
	GaugeAdd(metric string, value float64, labels map[string]string)
	Gauge(metric string, labels map[string]string) *prometheus.GaugeVec
	RegisterGauge(metric string, labels map[string]string) *prometheus.GaugeVec

	CounterAdd(metric string, value float64, labels map[string]string)
	Counter(metric string, labels map[string]string) *prometheus.CounterVec
	RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec

	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)
	Histogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
	RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec

	SendBatch(ops []operation.MetricOperation, labels map[string]string) error
	ApplyOperation(op operation.MetricOperation, commonLabels map[string]string)

	Grouped() GroupedStorage
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
