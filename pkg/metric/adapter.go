package metric

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/prometheus/client_golang/prometheus"

	klient "github.com/flant/kube-client/client"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type MetricStorage interface {
	RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec
	CounterAdd(metric string, value float64, labels map[string]string)
	RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec
	HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64)
}

// this adapter is used for kube-client
// it's deprecated, but don't know how to use it without breaking changes
//
//nolint:staticcheck
var _ klient.MetricStorage = (*MetricsAdapter)(nil)

type MetricsAdapter struct {
	Storage metricsstorage.Storage
	Logger  *log.Logger
}

func NewMetricsAdapter(storage metricsstorage.Storage, logger *log.Logger) *MetricsAdapter {
	return &MetricsAdapter{Storage: storage, Logger: logger}
}

// RegisterCounter registers a counter using the external storage
func (a *MetricsAdapter) RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec {
	// Use external storage to register the counter
	labelNames := utils.LabelNames(labels)
	_, err := a.Storage.RegisterCounter(metric, labelNames)
	if err != nil {
		a.Logger.Warn("failed to register counter metric", log.Err(err))
	}

	return nil
}

// CounterAdd adds a value to a counter using the external storage
func (a *MetricsAdapter) CounterAdd(metric string, value float64, labels map[string]string) {
	// Use external storage for the actual counter operation
	a.Storage.CounterAdd(metric, value, labels)
}

// RegisterHistogram registers a histogram using the external storage
func (a *MetricsAdapter) RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	// Use external storage to register the histogram
	labelNames := utils.LabelNames(labels)
	_, err := a.Storage.RegisterHistogram(metric, labelNames, buckets)
	if err != nil {
		a.Logger.Warn("failed to register histogram metric", log.Err(err))
	}

	return nil
}

// HistogramObserve observes a value in a histogram using the external storage
func (a *MetricsAdapter) HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64) {
	// Use external storage for the actual histogram operation
	a.Storage.HistogramObserve(metric, value, labels, buckets)
}
