package vault

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flant/shell-operator/pkg/metric"
	. "github.com/flant/shell-operator/pkg/utils/labels"
)

type GroupedVault struct {
	collectors            map[string]metric.ConstCollector
	mtx                   sync.Mutex
	registerer            prometheus.Registerer
	resolveMetricNameFunc func(name string) string
}

func NewGroupedVault(resolveMetricNameFunc func(name string) string) *GroupedVault {
	return &GroupedVault{
		collectors:            make(map[string]metric.ConstCollector),
		resolveMetricNameFunc: resolveMetricNameFunc,
	}
}

func (v *GroupedVault) Registerer() prometheus.Registerer {
	return v.registerer
}

func (v *GroupedVault) SetRegisterer(r prometheus.Registerer) {
	v.registerer = r
}

// ClearAllMetrics takes each collector in collectors and clear all metrics by group.
func (v *GroupedVault) ExpireGroupMetrics(group string) {
	v.mtx.Lock()
	for _, collector := range v.collectors {
		collector.ExpireGroupMetrics(group)
	}
	v.mtx.Unlock()
}

// ExpireGroupMetricByName gets a collector by its name and clears all metrics inside the collector by the group.
func (v *GroupedVault) ExpireGroupMetricByName(group, name string) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	collector, ok := v.collectors[metricName]
	if ok {
		collector.ExpireGroupMetrics(group)
	}
	v.mtx.Unlock()
}

func (v *GroupedVault) GetOrCreateCounterCollector(name string, labelNames []string) (*metric.ConstCounterCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[metricName]
	if !ok {
		collector = metric.NewConstCounterCollector(metricName, labelNames)
		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", metricName, labelNames, err)
		}
		v.collectors[metricName] = collector
	} else if !IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}
	if counter, ok := collector.(*metric.ConstCounterCollector); ok {
		return counter, nil
	}
	return nil, fmt.Errorf("counter %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) GetOrCreateGaugeCollector(name string, labelNames []string) (*metric.ConstGaugeCollector, error) {
	metricName := v.resolveMetricNameFunc(name)
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[metricName]
	if !ok {
		collector = metric.NewConstGaugeCollector(metricName, labelNames)
		if err := v.registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", metricName, labelNames, err)
		}
		v.collectors[metricName] = collector
	} else if !IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
	}

	if gauge, ok := collector.(*metric.ConstGaugeCollector); ok {
		return gauge, nil
	}
	return nil, fmt.Errorf("gauge %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) CounterAdd(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)
	c, err := v.GetOrCreateCounterCollector(metricName, LabelNames(labels))
	if err != nil {
		log.Errorf("CounterAdd: %v", err)
		return
	}
	c.Add(group, value, labels)
}

func (v *GroupedVault) GaugeSet(group string, name string, value float64, labels map[string]string) {
	metricName := v.resolveMetricNameFunc(name)
	c, err := v.GetOrCreateGaugeCollector(metricName, LabelNames(labels))
	if err != nil {
		log.Errorf("GaugeSet: %v", err)
		return
	}
	c.Set(group, value, labels)
}
