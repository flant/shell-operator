package metric

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/prometheus/client_golang/prometheus"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/metrics"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

// this adapter is used for kube-client
// it's deprecated, but don't know how to use it without breaking changes
//
//nolint:staticcheck
var _ klient.MetricStorage = (*MetricsAdapter)(nil)

type MetricsAdapter struct {
	Storage metricsstorage.Storage
	Logger  *log.Logger
	prefix  string
}

func NewMetricsAdapter(storage metricsstorage.Storage, prefix string, logger *log.Logger) *MetricsAdapter {
	return &MetricsAdapter{Storage: storage, prefix: prefix, Logger: logger}
}

func (a *MetricsAdapter) metricName(name string) string {
	return metrics.ReplacePrefix(name, a.prefix)
}

// RegisterCounter registers a counter using the external storage
func (a *MetricsAdapter) RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec {
	// Use external storage to register the counter
	labelNames := utils.LabelNames(labels)
	_, err := a.Storage.RegisterCounter(a.metricName(metric), labelNames)
	if err != nil {
		a.Logger.Warn("failed to register counter metric", log.Err(err))
	}

	return nil
}

// CounterAdd adds a value to a counter using the external storage
func (a *MetricsAdapter) CounterAdd(metric string, value float64, labels map[string]string) {
	// Use external storage for the actual counter operation
	a.Storage.CounterAdd(a.metricName(metric), value, labels)
}

// RegisterHistogram registers a histogram using the external storage
func (a *MetricsAdapter) RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	// Use external storage to register the histogram
	labelNames := utils.LabelNames(labels)
	_, err := a.Storage.RegisterHistogram(a.metricName(metric), labelNames, buckets)
	if err != nil {
		a.Logger.Warn("failed to register histogram metric", log.Err(err))
	}

	return nil
}

// HistogramObserve observes a value in a histogram using the external storage
func (a *MetricsAdapter) HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64) {
	// Use external storage for the actual histogram operation
	a.Storage.HistogramObserve(a.metricName(metric), value, labels, buckets)
}
