package metrics_storage

import (
	"context"
	"fmt"

	utils "github.com/flant/shell-operator/pkg/utils/labels"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// MetricStorage is used to synchronously register metric values.
type MetricStorage struct {
	ctx    context.Context
	cancel context.CancelFunc

	MetricChan chan Metric
	MetricVecs map[string]MetricVec
	Prefix     string
}

func NewMetricStorage() *MetricStorage {
	return &MetricStorage{
		MetricChan: make(chan Metric, 1000),
		MetricVecs: make(map[string]MetricVec),
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
	go func() {
		for {
			select {
			case metric := <-m.MetricChan:
				metric.store(m)
			case <-m.ctx.Done():
				return
			}
		}
	}()
}

func (m *MetricStorage) SendGauge(metric string, value float64, labels map[string]string) {
	m.MetricChan <- NewGaugeMetric(m.Prefix+metric, value, labels)
}
func (m *MetricStorage) SendCounter(metric string, value float64, labels map[string]string) {
	m.MetricChan <- NewCounterMetric(m.Prefix+metric, value, labels)
}

func (m *MetricStorage) SendGaugeNoPrefix(metric string, value float64, labels map[string]string) {
	m.MetricChan <- NewGaugeMetric(metric, value, labels)
}
func (m *MetricStorage) SendCounterNoPrefix(metric string, value float64, labels map[string]string) {
	m.MetricChan <- NewCounterMetric(metric, value, labels)
}

func (m *MetricStorage) SendBatch(ops []MetricOperation, labels map[string]string) error {
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

type Metric interface {
	store(*MetricStorage)
}

type BaseMetric struct {
	Metric string
	Value  float64
	Labels map[string]string
}

func (metric *BaseMetric) LabelsNames() []string {
	variableLabelNames := make([]string, 0)
	for labelName := range metric.Labels {
		variableLabelNames = append(variableLabelNames, labelName)
	}
	return variableLabelNames
}

func (metric *BaseMetric) getOrCreateMetricVec(storage *MetricStorage, createVecFunc func() (prometheus.Collector, MetricVec)) MetricVec {
	var metricVec MetricVec
	var prometheusCollector prometheus.Collector
	var hasMetricVec bool

	metricVec, hasMetricVec = storage.MetricVecs[metric.Metric]

	if !hasMetricVec {
		prometheusCollector, metricVec = createVecFunc()

		log.WithField("operator.component", "metricsStorage").Infof("Create new metric %s", metric.Metric)

		prometheus.MustRegister(prometheusCollector)
		storage.MetricVecs[metric.Metric] = metricVec
	}

	return metricVec
}

type GaugeMetric struct {
	BaseMetric
}

func NewGaugeMetric(metric string, value float64, labels map[string]string) *GaugeMetric {
	return &GaugeMetric{BaseMetric{
		Metric: metric,
		Value:  value,
		Labels: labels,
	}}
}

type CounterMetric struct {
	BaseMetric
}

func NewCounterMetric(metric string, value float64, labels map[string]string) *CounterMetric {
	return &CounterMetric{BaseMetric{
		Metric: metric,
		Value:  value,
		Labels: labels,
	}}
}

func (metric *GaugeMetric) store(storage *MetricStorage) {
	metricVec := metric.getOrCreateMetricVec(storage, func() (prometheus.Collector, MetricVec) {
		prometheusVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metric.Metric,
				Help: metric.Metric,
			},
			metric.LabelsNames(),
		)
		return prometheusVec, NewMetricGaugeVec(prometheusVec, metric.Metric, metric.LabelsNames())
	})
	metricVec.UpdateValue(metric.Labels, metric.Value)
}

func (metric *CounterMetric) store(storage *MetricStorage) {
	metricVec := metric.getOrCreateMetricVec(storage, func() (prometheus.Collector, MetricVec) {
		prometheusVec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: metric.Metric,
				Help: metric.Metric,
			},
			metric.LabelsNames(),
		)
		return prometheusVec, NewMetricCounterVec(prometheusVec, metric.Metric, metric.LabelsNames())
	})
	metricVec.UpdateValue(metric.Labels, metric.Value)
}

type MetricGaugeVec struct {
	*prometheus.GaugeVec
	Name       string
	LabelNames []string
}

func NewMetricGaugeVec(gauge *prometheus.GaugeVec, name string, labelNames []string) *MetricGaugeVec {
	metricGaugeVec := &MetricGaugeVec{gauge, name, make([]string, 0)}
	metricGaugeVec.LabelNames = append(metricGaugeVec.LabelNames, labelNames...)
	return metricGaugeVec
}

type MetricCounterVec struct {
	*prometheus.CounterVec
	Name       string
	LabelNames []string
}

func NewMetricCounterVec(counter *prometheus.CounterVec, name string, labelNames []string) *MetricCounterVec {
	metricCounterVec := &MetricCounterVec{counter, name, make([]string, 0)}
	metricCounterVec.LabelNames = append(metricCounterVec.LabelNames, labelNames...)
	return metricCounterVec
}

type MetricVec interface {
	UpdateValue(labels prometheus.Labels, value float64)
}

func (metricVec *MetricGaugeVec) UpdateValue(labels prometheus.Labels, value float64) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric %s %v update with %v error: %v", metricVec.Name, metricVec.LabelNames, labels, r)
		}
	}()
	metricVec.With(labels).Set(value)
}
func (metricVec *MetricCounterVec) UpdateValue(labels prometheus.Labels, value float64) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("operator.component", "metricsStorage").
				Errorf("Metric %s %v update with %v error: %v", metricVec.Name, metricVec.LabelNames, labels, r)
		}
	}()
	metricVec.With(labels).Add(value)
}
