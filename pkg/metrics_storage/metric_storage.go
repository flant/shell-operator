package metrics_storage

import (
	"context"
	"fmt"

	utils "github.com/flant/shell-operator/pkg/utils/labels"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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
}

func NewMetricStorage() *MetricStorage {
	return &MetricStorage{
		Gauges:           make(map[string]*prometheus.GaugeVec),
		Counters:         make(map[string]*prometheus.CounterVec),
		Histograms:       make(map[string]*prometheus.HistogramVec),
		HistogramBuckets: make(map[string][]float64),
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

func (m *MetricStorage) SendBatch(ops []MetricOperation, labels map[string]string) error {
	if m == nil {
		return nil
	}
	// Apply metric operations
	for _, metricOp := range ops {
		labels := utils.MergeLabels(metricOp.Labels, labels)

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

func LabelNames(labels map[string]string) []string {
	names := make([]string, 0)
	for labelName := range labels {
		names = append(names, labelName)
	}
	return names
}
