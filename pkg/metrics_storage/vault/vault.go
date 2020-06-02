package vault

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/utils/labels"
)

type GroupedVault struct {
	collectors map[string]ConstMetricCollector
	mtx        sync.Mutex
}

func NewGroupedVault() *GroupedVault {
	return &GroupedVault{
		collectors: make(map[string]ConstMetricCollector),
	}
}

// ClearMissingMetrics takes each collector in collector by its name and clear missing metrics by group.
func (v *GroupedVault) ClearMissingMetrics(group string, metricLabels map[string][]map[string]string) {
	v.mtx.Lock()
	defer v.mtx.Unlock()
	for name, collector := range v.collectors {
		if labelSet, ok := metricLabels[name]; ok {
			collector.ClearMissingMetrics(group, labelSet)
		}
	}
}

func (v *GroupedVault) GetOrCreateCounterCollector(name string, labelNames []string) (*ConstCounterCollector, error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[name]
	if !ok {
		collector = NewConstCounterCollector(name, labelNames)
		if err := prometheus.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", name, labelNames, err)
		}
		v.collectors[name] = collector
	}
	if counter, ok := collector.(*ConstCounterCollector); ok {
		return counter, nil
	}
	return nil, fmt.Errorf("counter %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) GetOrCreateGaugeCollector(name string, labelNames []string) (*ConstGaugeCollector, error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[name]
	if !ok {
		collector = NewConstGaugeCollector(name, labelNames)
		if err := prometheus.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", name, labelNames, err)
		}
		v.collectors[name] = collector
	}
	if gauge, ok := collector.(*ConstGaugeCollector); ok {
		return gauge, nil
	}
	return nil, fmt.Errorf("gauge %v collector requested, but %s %v collector exists", labelNames, collector.Type(), collector.LabelNames())
}

func (v *GroupedVault) CounterAdd(group string, name string, value float64, labels map[string]string) {
	c, err := v.GetOrCreateCounterCollector(name, LabelNames(labels))
	if err != nil {
		log.Errorf("CounterAdd: %v", err)
		return
	}
	c.Add(group, value, labels)
}

func (v *GroupedVault) GaugeSet(group string, name string, value float64, labels map[string]string) {
	c, err := v.GetOrCreateGaugeCollector(name, LabelNames(labels))
	if err != nil {
		log.Errorf("CounterAdd: %v", err)
		return
	}
	c.Set(group, value, labels)
}
