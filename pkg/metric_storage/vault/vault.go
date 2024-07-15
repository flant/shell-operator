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
	Registerer prometheus.Registerer
}

func NewGroupedVault() *GroupedVault {
	return &GroupedVault{
		collectors: make(map[string]ConstMetricCollector),
	}
}

// ClearAllMetrics takes each collector in collectors and clear all metrics by group.
func (v *GroupedVault) ExpireGroupMetrics(group string) {
	v.mtx.Lock()
	defer v.mtx.Unlock()
	for _, collector := range v.collectors {
		collector.ExpireGroupMetrics(group)
	}
}

func (v *GroupedVault) GetOrCreateCounterCollector(name string, labelNames []string) (*ConstCounterCollector, error) {
	v.mtx.Lock()
	defer v.mtx.Unlock()
	collector, ok := v.collectors[name]
	if !ok {
		collector = NewConstCounterCollector(name, labelNames)
		if err := v.Registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("counter '%s' %v registration: %v", name, labelNames, err)
		}
		v.collectors[name] = collector
	} else if !IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
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
		if err := v.Registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("gauge '%s' %v registration: %v", name, labelNames, err)
		}
		v.collectors[name] = collector
	} else if !IsSubset(collector.LabelNames(), labelNames) {
		collector.UpdateLabels(labelNames)
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
		log.Errorf("GaugeSet: %v", err)
		return
	}
	c.Set(group, value, labels)
}
