package metrics_storage

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/romana/rlog"
)

// Metrics collection methods
// Antiopa generates the following metrics:
// hooks errors
// - shell_operator_hook_errors{hook="hook_name"} counter increases when hook fails
// - shell_operator_hook_beareable_errors{hook="xxx"}
// a work counter
// - shell_operator_live_ticks counter increases every 5 sec. while shell_operator runs.
// queue length
// - shell_operator_tasks_queue_length
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
	for labelName, _ := range metric.Labels {
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

		rlog.Infof("MSTOR Create new metric %s", metric.Metric)

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
	for _, labelName := range labelNames {
		metricGaugeVec.LabelNames = append(metricGaugeVec.LabelNames, labelName)
	}
	return metricGaugeVec
}

type MetricCounterVec struct {
	*prometheus.CounterVec
	Name       string
	LabelNames []string
}

func NewMetricCounterVec(counter *prometheus.CounterVec, name string, labelNames []string) *MetricCounterVec {
	metricCounterVec := &MetricCounterVec{counter, name, make([]string, 0)}
	for _, labelName := range labelNames {
		metricCounterVec.LabelNames = append(metricCounterVec.LabelNames, labelName)
	}
	return metricCounterVec
}

type MetricVec interface {
	UpdateValue(labels prometheus.Labels, value float64)
}

func (metricVec *MetricGaugeVec) UpdateValue(labels prometheus.Labels, value float64) {
	defer func() {
		if r := recover(); r != nil {
			rlog.Errorf("MSTOR Panic! Metric %s %v update with %v error: %v", metricVec.Name, metricVec.LabelNames, labels, r)
		}
	}()
	metricVec.With(labels).Set(value)
}
func (metricVec *MetricCounterVec) UpdateValue(labels prometheus.Labels, value float64) {
	defer func() {
		if r := recover(); r != nil {
			rlog.Errorf("MSTOR Panic! Metric %s %v update with %v error: %v", metricVec.Name, metricVec.LabelNames, labels, r)
		}
	}()
	metricVec.With(labels).Add(value)
}

func Init() *MetricStorage {
	return NewMetricStorage()
}

// Структура MetricStorage - регистратор результатов
type MetricStorage struct {
	MetricChan chan Metric
	MetricVecs map[string]MetricVec
	//EmptyConstLabels map[string]string
}

func NewMetricStorage() *MetricStorage {
	return &MetricStorage{
		MetricChan: make(chan Metric, 1000),
		MetricVecs: make(map[string]MetricVec),
		//EmptyConstLabels: make(map[string]string),
	}
}

func (storage *MetricStorage) Run() {
	for {
		select {
		case metric := <-storage.MetricChan:
			metric.store(storage)
		}
	}
}

func (storage *MetricStorage) SendGaugeMetric(metric string, value float64, labels map[string]string) {
	storage.MetricChan <- NewGaugeMetric(metric, value, labels)
}
func (storage *MetricStorage) SendCounterMetric(metric string, value float64, labels map[string]string) {
	storage.MetricChan <- NewCounterMetric(metric, value, labels)
}
