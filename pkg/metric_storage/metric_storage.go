package metricstorage

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	"github.com/flant/shell-operator/pkg/metric_storage/vault"
	. "github.com/flant/shell-operator/pkg/utils/labels"
)

const (
	PrefixTemplate = "{PREFIX}"
)

// MetricStorage is used to register metric values.
type MetricStorage struct {
	ctx    context.Context
	cancel context.CancelFunc

	Prefix string

	Counters         map[string]*prometheus.CounterVec
	Gauges           map[string]*prometheus.GaugeVec
	Histograms       map[string]*prometheus.HistogramVec
	HistogramBuckets map[string][]float64

	countersLock   sync.RWMutex
	gaugesLock     sync.RWMutex
	histogramsLock sync.RWMutex

	groupedVault *vault.GroupedVault

	Registry   *prometheus.Registry
	Gatherer   prometheus.Gatherer
	Registerer prometheus.Registerer

	logger *log.Logger
}

func NewMetricStorage(ctx context.Context, prefix string, newRegistry bool, logger *log.Logger) *MetricStorage {
	cctx, cancel := context.WithCancel(ctx)
	m := &MetricStorage{
		ctx:    cctx,
		cancel: cancel,

		Prefix:           prefix,
		Gauges:           make(map[string]*prometheus.GaugeVec),
		Counters:         make(map[string]*prometheus.CounterVec),
		Histograms:       make(map[string]*prometheus.HistogramVec),
		HistogramBuckets: make(map[string][]float64),
		Gatherer:         prometheus.DefaultGatherer,
		Registerer:       prometheus.DefaultRegisterer,

		logger: logger.With("operator.component", "metricsStorage"),
	}
	m.groupedVault = vault.NewGroupedVault(m.resolveMetricName)
	m.groupedVault.SetRegisterer(m.Registerer)

	if newRegistry {
		m.Registry = prometheus.NewRegistry()
		m.Gatherer = m.Registry
		m.Registerer = m.Registry
		m.groupedVault.SetRegisterer(m.Registry)
	}

	return m
}

func (m *MetricStorage) Grouped() metric.GroupedStorage {
	return m.groupedVault
}

func (m *MetricStorage) resolveMetricName(name string) string {
	if strings.Contains(name, PrefixTemplate) {
		return strings.Replace(name, PrefixTemplate, m.Prefix, 1)
	}
	return name
}

// Gauges

func (m *MetricStorage) GaugeSet(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Metric gauge set",
				slog.String("metric", m.resolveMetricName(metric)),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()

	m.Gauge(metric, labels).With(labels).Set(value)
}

func (m *MetricStorage) GaugeAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Metric gauge add",
				slog.String("metric", m.resolveMetricName(metric)),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()

	m.Gauge(metric, labels).With(labels).Add(value)
}

// Gauge return saved or register a new gauge.
func (m *MetricStorage) Gauge(metric string, labels map[string]string) *prometheus.GaugeVec {
	m.gaugesLock.RLock()
	vec, ok := m.Gauges[metric]
	m.gaugesLock.RUnlock()
	if ok {
		return vec
	}

	return m.RegisterGauge(metric, labels)
}

// RegisterGauge registers a gauge.
func (m *MetricStorage) RegisterGauge(metric string, labels map[string]string) *prometheus.GaugeVec {
	metricName := m.resolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Create metric gauge",
				slog.String("metric", m.resolveMetricName(metric)),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()

	m.gaugesLock.Lock()
	defer m.gaugesLock.Unlock()
	// double check
	vec, ok := m.Gauges[metric]
	if ok {
		return vec
	}

	m.logger.Info("Create metric gauge", slog.String("name", metricName))
	vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricName,
			Help: metricName,
		},
		LabelNames(labels),
	)
	m.Registerer.Register(vec)
	m.Gauges[metric] = vec
	return vec
}

// Counters

func (m *MetricStorage) CounterAdd(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Metric counter add",
				slog.String("metric", m.resolveMetricName(metric)),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()
	m.Counter(metric, labels).With(labels).Add(value)
}

// Counter ...
func (m *MetricStorage) Counter(metric string, labels map[string]string) *prometheus.CounterVec {
	m.countersLock.RLock()
	vec, ok := m.Counters[metric]
	m.countersLock.RUnlock()
	if ok {
		return vec
	}

	return m.RegisterCounter(metric, labels)
}

// RegisterCounter registers a counter.
func (m *MetricStorage) RegisterCounter(metric string, labels map[string]string) *prometheus.CounterVec {
	metricName := m.resolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Create metric counter",
				slog.String("metric", metricName),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()

	m.countersLock.Lock()
	defer m.countersLock.Unlock()
	// double check
	vec, ok := m.Counters[metric]
	if ok {
		return vec
	}

	m.logger.Info("Create metric counter", slog.String("name", metricName))
	vec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricName,
			Help: metricName,
		},
		LabelNames(labels),
	)
	m.Registerer.Register(vec)
	m.Counters[metric] = vec
	return vec
}

// Histograms

func (m *MetricStorage) HistogramObserve(metric string, value float64, labels map[string]string, buckets []float64) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Metric histogram observe",
				slog.String("metric", m.resolveMetricName(metric)),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()
	m.Histogram(metric, labels, buckets).With(labels).Observe(value)
}

func (m *MetricStorage) Histogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	m.histogramsLock.RLock()
	vec, ok := m.Histograms[metric]
	m.histogramsLock.RUnlock()
	if ok {
		return vec
	}
	return m.RegisterHistogram(metric, labels, buckets)
}

func (m *MetricStorage) RegisterHistogram(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	metricName := m.resolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Create metric histogram",
				slog.String("metric", metricName),
				slog.String("labels", strings.Join(LabelNames(labels), ", ")),
				slog.String("labels_values", fmt.Sprintf("%v", labels)),
				slog.String("recover", fmt.Sprintf("%v", r)))
		}
	}()

	m.histogramsLock.Lock()
	defer m.histogramsLock.Unlock()
	// double check
	vec, ok := m.Histograms[metric]
	if ok {
		return vec
	}

	m.logger.Info("Create metric histogram", slog.String("name", metricName))
	b, has := m.HistogramBuckets[metric]
	// This shouldn't happen except when entering this concurrently
	// If there are buckets for this histogram about to be registered, keep them
	// Otherwise, use the new buckets.
	// No need to check for nil or empty slice, as the p8s lib will use DefBuckets
	// (https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#HistogramOpts)
	if has {
		buckets = b
	}

	vec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricName,
		Help:    metricName,
		Buckets: buckets,
	}, LabelNames(labels))

	m.Registerer.Register(vec)
	m.Histograms[metric] = vec
	return vec
}

// Batch operations for metrics from hooks

func (m *MetricStorage) sendBatchV0(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}
	// Apply metric operations
	for _, metricOp := range ops {
		labels := MergeLabels(metricOp.Labels, labels)

		if metricOp.Action == "add" && metricOp.Value != nil {
			m.CounterAdd(metricOp.Name, *metricOp.Value, labels)
			continue
		}
		if metricOp.Action == "set" && metricOp.Value != nil {
			m.GaugeSet(metricOp.Name, *metricOp.Value, labels)
			continue
		}
		if metricOp.Action == "observe" && metricOp.Value != nil && metricOp.Buckets != nil {
			m.HistogramObserve(metricOp.Name, *metricOp.Value, labels, metricOp.Buckets)
			continue
		}
		return fmt.Errorf("no operation in metric from module hook, name=%s", metricOp.Name)
	}
	return nil
}

func (m *MetricStorage) SendBatch(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}

	err := operation.ValidateOperations(ops)
	if err != nil {
		return err
	}

	// Group operations by 'Group' value.
	groupedOps := make(map[string][]operation.MetricOperation)
	nonGroupedOps := make([]operation.MetricOperation, 0)

	for _, op := range ops {
		if op.Group == "" {
			nonGroupedOps = append(nonGroupedOps, op)
			continue
		}
		if _, ok := groupedOps[op.Group]; !ok {
			groupedOps[op.Group] = make([]operation.MetricOperation, 0)
		}
		groupedOps[op.Group] = append(groupedOps[op.Group], op)
	}

	// Expire each group and apply new metric operations.
	for group, ops := range groupedOps {
		m.applyGroupOperations(group, ops, labels)
	}

	// Send non-grouped metrics.
	err = m.sendBatchV0(nonGroupedOps, labels)
	if err != nil {
		return err
	}

	return nil
}

func (m *MetricStorage) ApplyOperation(op operation.MetricOperation, commonLabels map[string]string) {
	labels := MergeLabels(op.Labels, commonLabels)

	if op.Action == "add" && op.Value != nil {
		m.CounterAdd(op.Name, *op.Value, labels)
		return
	}
	//nolint:staticcheck
	if op.Add != nil {
		m.CounterAdd(op.Name, *op.Add, labels)
		return
	}
	if op.Action == "set" && op.Value != nil {
		m.GaugeSet(op.Name, *op.Value, labels)
		return
	}
	//nolint:staticcheck
	if op.Set != nil {
		m.GaugeSet(op.Name, *op.Set, labels)
		return
	}
	if op.Action == "observe" && op.Value != nil && op.Buckets != nil {
		m.HistogramObserve(op.Name, *op.Value, labels, op.Buckets)
	}
}

// applyGroupOperations set metrics for group to a new state defined by ops.
func (m *MetricStorage) applyGroupOperations(group string, ops []operation.MetricOperation, commonLabels map[string]string) {
	// Implicitly expire all metrics for group.
	m.groupedVault.ExpireGroupMetrics(group)

	// Apply metric operations one-by-one.
	for _, op := range ops {
		if op.Action == "expire" {
			m.groupedVault.ExpireGroupMetrics(group)
			continue
		}
		labels := MergeLabels(op.Labels, commonLabels)
		if op.Action == "add" && op.Value != nil {
			m.groupedVault.CounterAdd(group, op.Name, *op.Value, labels)
		}
		//nolint:staticcheck
		if op.Add != nil {
			m.groupedVault.CounterAdd(group, op.Name, *op.Add, labels)
		}
		if op.Action == "set" && op.Value != nil {
			m.groupedVault.GaugeSet(group, op.Name, *op.Value, labels)
		}
		//nolint:staticcheck
		if op.Set != nil {
			m.groupedVault.GaugeSet(group, op.Name, *op.Set, labels)
		}
	}
}

func (m *MetricStorage) Handler() http.Handler {
	if m.Registry == nil {
		return promhttp.Handler()
	}

	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		Registry: m.Registry,
	})
}

func (m *MetricStorage) Cleanup() {
	for _, vec := range m.Counters {
		m.Registerer.Unregister(vec)
	}
	for _, vec := range m.Gauges {
		m.Registerer.Unregister(vec)
	}
	for _, vec := range m.Histograms {
		m.Registerer.Unregister(vec)
	}
}
