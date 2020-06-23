package metric_storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	"github.com/flant/shell-operator/pkg/metric_storage/vault"
	. "github.com/flant/shell-operator/pkg/utils/labels"
)

const (
	HistogramDefaultStart float64 = 0.0
	HistogramDefaultWidth float64 = 2
	HistogramDefaultCount int     = 20
	PrefixTemplate                = "{PREFIX}"
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

	GroupedVault *vault.GroupedVault
}

func NewMetricStorage() *MetricStorage {
	return &MetricStorage{
		Gauges:           make(map[string]*prometheus.GaugeVec),
		Counters:         make(map[string]*prometheus.CounterVec),
		Histograms:       make(map[string]*prometheus.HistogramVec),
		HistogramBuckets: make(map[string][]float64),
		GroupedVault:     vault.NewGroupedVault(),
	}
}

func (m *MetricStorage) WithContext(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
}

func (m *MetricStorage) WithPrefix(prefix string) {
	m.Prefix = prefix
}

func (m *MetricStorage) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *MetricStorage) Start() {
	//go func() {
	//	<-m.ctx.Done()
	//	return
	//}()
}

func (m *MetricStorage) ResolveMetricName(name string) string {
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
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric gauge set %s %v with %v: %v", m.ResolveMetricName(metric), LabelNames(labels), labels, r)
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
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric gauge add %s %v with %v: %v", m.ResolveMetricName(metric), LabelNames(labels), labels, r)
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
	metricName := m.ResolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricStorage").
				Errorf("Create metric gauge %s %v with %v: %v", metricName, LabelNames(labels), labels, r)
		}
	}()

	m.gaugesLock.Lock()
	defer m.gaugesLock.Unlock()
	// double check
	vec, ok := m.Gauges[metric]
	if ok {
		return vec
	}

	log.WithField("operator.component", "metricStorage").
		Infof("Create metric gauge %s", metricName)
	vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricName,
			Help: metricName,
		},
		LabelNames(labels),
	)
	prometheus.MustRegister(vec)
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
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric counter add %s %v with %v: %v", m.ResolveMetricName(metric), LabelNames(labels), labels, r)
		}
	}()
	m.Counter(metric, labels).With(labels).Add(value)
}

// Counter
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
	metricName := m.ResolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricStorage").
				Errorf("Create metric counter %s %v with %v: %v", metricName, LabelNames(labels), labels, r)
		}
	}()

	m.countersLock.Lock()
	defer m.countersLock.Unlock()
	// double check
	vec, ok := m.Counters[metric]
	if ok {
		return vec
	}

	log.WithField("operator.component", "metricStorage").
		Infof("Create metric counter %s", metricName)
	vec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricName,
			Help: metricName,
		},
		LabelNames(labels),
	)
	prometheus.MustRegister(vec)
	m.Counters[metric] = vec
	return vec
}

// Histograms

func (m *MetricStorage) HistogramObserve(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric histogram observe %s %v with %v: %v", m.ResolveMetricName(metric), LabelNames(labels), labels, r)
		}
	}()
	m.Histogram(metric, labels).With(labels).Observe(value)
}

func (m *MetricStorage) HistogramDefineBuckets(metric string, buckets []float64) {
	if m == nil {
		return
	}
	m.HistogramBuckets[metric] = buckets
}

func (m *MetricStorage) Histogram(metric string, labels map[string]string) *prometheus.HistogramVec {
	m.histogramsLock.RLock()
	vec, ok := m.Histograms[metric]
	m.histogramsLock.RUnlock()
	if ok {
		return vec
	}

	return m.RegisterHistogram(metric, labels)
}

func (m *MetricStorage) RegisterHistogram(metric string, labels map[string]string) *prometheus.HistogramVec {
	metricName := m.ResolveMetricName(metric)

	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Create metric histogram %s %v with %v: %v", metricName, LabelNames(labels), labels, r)
		}
	}()

	m.histogramsLock.Lock()
	defer m.histogramsLock.Unlock()
	// double check
	vec, ok := m.Histograms[metric]
	if ok {
		return vec
	}

	log.WithField("operator.component", "metricsStorage").
		Infof("Create metric histogram %s", metricName)
	buckets, has := m.HistogramBuckets[metric]
	if !has {
		buckets = prometheus.LinearBuckets(HistogramDefaultStart, HistogramDefaultWidth, HistogramDefaultCount)
	}
	vec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricName,
		Help:    metricName,
		Buckets: buckets,
	}, LabelNames(labels))

	prometheus.MustRegister(vec)
	m.Histograms[metric] = vec
	return vec
}

func (m *MetricStorage) RegisterHistogramWithBuckets(metric string, labels map[string]string, buckets []float64) *prometheus.HistogramVec {
	m.HistogramDefineBuckets(metric, buckets)
	return m.RegisterHistogram(metric, labels)
}

// Batch operations for metrics from hooks

func (m *MetricStorage) SendBatchV0(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}
	// Apply metric operations
	for _, metricOp := range ops {
		labels := MergeLabels(metricOp.Labels, labels)

		if metricOp.Add != nil {
			m.CounterAdd(metricOp.Name, *metricOp.Add, labels)
			continue
		}
		if metricOp.Set != nil {
			m.GaugeSet(metricOp.Name, *metricOp.Set, labels)
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

	// Group ops by 'Group'.
	var groupedOps = make(map[string][]operation.MetricOperation)
	var nonGroupedOps = make([]operation.MetricOperation, 0)

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

	// Apply metric operations for each group
	for group, ops := range groupedOps {
		// clean group
		m.ApplyGroupOperations(group, ops, labels)
	}

	// backward compatibility — send metrics without cleaning
	err := m.SendBatchV0(nonGroupedOps, labels)
	if err != nil {
		return err
	}

	return nil
}

func (m *MetricStorage) ApplyOperation(op operation.MetricOperation, commonLabels map[string]string) {
	labels := MergeLabels(op.Labels, commonLabels)

	if op.Add != nil {
		m.CounterAdd(op.Name, *op.Add, labels)
		return
	}
	if op.Set != nil {
		m.GaugeSet(op.Name, *op.Set, labels)
		return
	}
}

func (m *MetricStorage) ApplyGroupOperations(group string, ops []operation.MetricOperation, commonLabels map[string]string) {
	// Gather labels to clear absent metrics by label values
	var metricLabels = make(map[string][]map[string]string)
	for _, op := range ops {
		if _, ok := metricLabels[op.Name]; !ok {
			metricLabels[op.Name] = make([]map[string]string, 0)
		}
		metricLabels[op.Name] = append(metricLabels[op.Name], MergeLabels(op.Labels, commonLabels))
	}
	m.GroupedVault.ClearMissingMetrics(group, metricLabels)

	// Apply metric operations
	for _, op := range ops {
		labels := MergeLabels(op.Labels, commonLabels)
		if op.Add != nil {
			m.GroupedVault.CounterAdd(group, op.Name, *op.Add, labels)
			continue
		}
		if op.Set != nil {
			m.GroupedVault.GaugeSet(group, op.Name, *op.Set, labels)
			continue
		}
	}
}
