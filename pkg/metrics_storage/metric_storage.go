package metrics_storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/metrics_storage/operation"
	"github.com/flant/shell-operator/pkg/metrics_storage/vault"
	. "github.com/flant/shell-operator/pkg/utils/labels"
)

const (
	HistogramDefaultStart float64 = 0.0
	HistogramDefaultWidth float64 = 2
	HistogramDefaultCount int     = 20
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

	countersLock   sync.Mutex
	gaugesLock     sync.Mutex
	histogramsLock sync.Mutex

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

func (m *MetricStorage) SendGauge(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.SendGaugeNoPrefix(m.Prefix+metric, value, labels)
}
func (m *MetricStorage) SendGaugeNoPrefix(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric gauge %s %v update with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()

	m.GetOrDefineGauge(metric, labels).With(labels).Set(value)
}

func (m *MetricStorage) GetOrDefineGauge(metric string, labels map[string]string) *prometheus.GaugeVec {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric gauge %s %v create with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()

	log.WithField("operator.component", "metricsStorage").
		Debugf("Get metric gauge %s", metric)
	m.gaugesLock.Lock()
	defer m.gaugesLock.Unlock()

	vec, ok := m.Gauges[metric]

	// create and register CounterVec
	if !ok {
		vec = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metric,
				Help: metric,
			},
			LabelNames(labels),
		)
		log.WithField("operator.component", "metricsStorage").
			Infof("Create new metric gauge %s", metric)
		prometheus.MustRegister(vec)
		m.Gauges[metric] = vec
	}

	return vec
}

func (m *MetricStorage) SendCounter(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.SendCounterNoPrefix(m.Prefix+metric, value, labels)
}

func (m *MetricStorage) SendCounterNoPrefix(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric %s %v update with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()

	m.GetOrDefineCounter(metric, labels).With(labels).Add(value)
}

func (m *MetricStorage) GetOrDefineCounter(metric string, labels map[string]string) *prometheus.CounterVec {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric counter %s %v create with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()

	log.WithField("operator.component", "metricsStorage").
		Debugf("Get metric counter %s", metric)
	m.countersLock.Lock()
	defer m.countersLock.Unlock()
	vec, ok := m.Counters[metric]

	// create and register CounterVec
	if !ok {
		vec = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metric,
				Help: metric,
			},
			LabelNames(labels),
		)
		log.WithField("operator.component", "metricsStorage").
			Infof("Create new metric counter %s", metric)
		prometheus.MustRegister(vec)
		m.Counters[metric] = vec
	}

	return vec
}

func (m *MetricStorage) ObserveHistogramNoPrefix(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric histogram %s %v update with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()
	m.GetOrDefineHistogram(metric, labels).With(labels).Observe(value)
}

func (m *MetricStorage) ObserveHistogram(metric string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.ObserveHistogramNoPrefix(m.Prefix+metric, value, labels)
}

func (m *MetricStorage) DefineHistogramBuckets(metric string, buckets []float64) {
	m.DefineHistogramBucketsNoPrefix(m.Prefix+metric, buckets)
}

func (m *MetricStorage) DefineHistogramBucketsNoPrefix(metric string, buckets []float64) {
	if m == nil {
		return
	}
	m.HistogramBuckets[metric] = buckets
}

func (m *MetricStorage) GetOrDefineHistogram(metric string, labels map[string]string) *prometheus.HistogramVec {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric histogram %s %v create with %v error: %v", metric, LabelNames(labels), labels, r)
		}
	}()

	log.WithField("operator.component", "metricsStorage").
		Debugf("Get metric histogram %s", metric)

	m.histogramsLock.Lock()
	defer m.histogramsLock.Unlock()
	vec, ok := m.Histograms[metric]
	if !ok {
		buckets, has := m.HistogramBuckets[metric]
		if !has {
			buckets = prometheus.LinearBuckets(HistogramDefaultStart, HistogramDefaultWidth, HistogramDefaultCount)
		}
		vec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metric,
			Help:    metric,
			Buckets: buckets,
		}, LabelNames(labels))

		log.WithField("operator.component", "metricsStorage").
			Infof("Create new metric histogram %s", metric)
		prometheus.MustRegister(vec)
		m.Histograms[metric] = vec
	}
	return vec
}

func (m *MetricStorage) SendBatchV0(ops []operation.MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}
	// Apply metric operations
	for _, metricOp := range ops {
		labels := MergeLabels(metricOp.Labels, labels)

		if metricOp.Add != nil {
			m.SendCounterNoPrefix(metricOp.Name, *metricOp.Add, labels)
			continue
		}
		if metricOp.Set != nil {
			m.SendGaugeNoPrefix(metricOp.Name, *metricOp.Set, labels)
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
		m.SendCounterNoPrefix(op.Name, *op.Add, labels)
		return
	}
	if op.Set != nil {
		m.SendGaugeNoPrefix(op.Name, *op.Set, labels)
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
