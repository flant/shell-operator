package vault

import (
	"hash/fnv"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	. "github.com/flant/shell-operator/pkg/utils/labels"
)

type ConstMetricCollector interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)
	Type() string
	LabelNames() []string
	Name() string
	ClearMissingMetrics(group string, labelSet []map[string]string)
	ClearAllMetrics(group string)
}

var (
	_ ConstMetricCollector = (*ConstCounterCollector)(nil)
	_ ConstMetricCollector = (*ConstGaugeCollector)(nil)
)

type GroupedCounterMetric struct {
	Value       uint64
	LabelValues []string
	Group       string
}

type GroupedGaugeMetric struct {
	Value       float64
	LabelValues []string
	Group       string
}

type ConstCounterCollector struct {
	mtx sync.RWMutex

	collection map[uint64]GroupedCounterMetric
	desc       *prometheus.Desc
	name       string
	labelNames []string
}

func NewConstCounterCollector(name string, labelNames []string) *ConstCounterCollector {
	desc := prometheus.NewDesc(name, name, labelNames, nil)
	return &ConstCounterCollector{
		name:       name,
		labelNames: labelNames,
		desc:       desc,
		collection: make(map[uint64]GroupedCounterMetric),
	}
}

// Add increases a counter metric by a value. Metric is identified by label values and a group.
func (c *ConstCounterCollector) Add(group string, value float64, labels map[string]string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	labelValues := LabelValues(labels, c.labelNames)
	labelsHash := HashLabelValues(labelValues)

	// TODO add group to hash
	storedMetric, ok := c.collection[labelsHash]
	if !ok {
		storedMetric = GroupedCounterMetric{
			Value:       uint64(value),
			LabelValues: labelValues,
			Group:       group,
		}
	} else {
		atomic.AddUint64(&storedMetric.Value, uint64(value))
	}

	c.collection[labelsHash] = storedMetric
}

func (c *ConstCounterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstCounterCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, s := range c.collection {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(s.Value), s.LabelValues...)
	}
}

func (c *ConstCounterCollector) Type() string {
	return "counter"
}

func (c *ConstCounterCollector) LabelNames() []string {
	return c.labelNames
}

func (c *ConstCounterCollector) Name() string {
	return c.name
}

// ClearMissingMetrics deletes metrics from collection with unmatched hashes of labelValues.
func (c *ConstCounterCollector) ClearMissingMetrics(group string, labelSet []map[string]string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	hashedOps := make(map[uint64]struct{})
	for _, labels := range labelSet {
		hashedOps[HashLabelValues(LabelValues(labels, c.labelNames))] = struct{}{}
	}

	for hash, m := range c.collection {
		if m.Group != group {
			continue
		}
		if _, ok := hashedOps[hash]; !ok {
			delete(c.collection, hash)
		}
	}
}

// ClearAllMetrics deletes all metrics from collection with matched group.
func (c *ConstCounterCollector) ClearAllMetrics(group string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for hash, m := range c.collection {
		if m.Group == group {
			delete(c.collection, hash)
		}
	}
}

type ConstGaugeCollector struct {
	mtx sync.RWMutex

	name       string
	labelNames []string
	desc       *prometheus.Desc
	collection map[uint64]GroupedGaugeMetric
}

func NewConstGaugeCollector(name string, labelNames []string) *ConstGaugeCollector {
	desc := prometheus.NewDesc(name, name, labelNames, nil)
	return &ConstGaugeCollector{
		name:       name,
		labelNames: labelNames,
		desc:       desc,
		collection: make(map[uint64]GroupedGaugeMetric),
	}
}

func (c *ConstGaugeCollector) Set(group string, value float64, labels map[string]string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	labelValues := LabelValues(labels, c.labelNames)
	labelsHash := HashLabelValues(labelValues)

	storedMetric, ok := c.collection[labelsHash]
	if !ok {
		storedMetric = GroupedGaugeMetric{
			Value:       value,
			LabelValues: labelValues,
			Group:       group,
		}
	}

	storedMetric.Value = value
	c.collection[labelsHash] = storedMetric
}

func (c *ConstGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *ConstGaugeCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, s := range c.collection {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, s.Value, s.LabelValues...)
	}
}

func (c *ConstGaugeCollector) Type() string {
	return "gauge"
}

func (c *ConstGaugeCollector) LabelNames() []string {
	return c.labelNames
}

func (c *ConstGaugeCollector) Name() string {
	return c.name
}

// ClearGroup removes all metrics for a group
// TODO ClearGroup

// Clear removes all metrics from this collector
//TODO

// ClearMissingMetrics deletes metrics from collection with unmatched hashes of labelValues.
func (c *ConstGaugeCollector) ClearMissingMetrics(group string, labelSet []map[string]string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	hashedOps := make(map[uint64]struct{})
	for _, labels := range labelSet {
		hashedOps[HashLabelValues(LabelValues(labels, c.labelNames))] = struct{}{}
	}

	for hash, m := range c.collection {
		if m.Group != group {
			continue
		}
		if _, ok := hashedOps[hash]; !ok {
			delete(c.collection, hash)
		}
	}
}

// ClearAllMetrics deletes all metrics from collection with matched group.
func (c *ConstGaugeCollector) ClearAllMetrics(group string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for hash, m := range c.collection {
		if m.Group == group {
			delete(c.collection, hash)
		}
	}
}

const labelsSeparator = byte(255)

func HashLabelValues(labelValues []string) uint64 {
	hasher := fnv.New64a()
	for _, labelValue := range labelValues {
		_, _ = hasher.Write([]byte(labelValue))
		_, _ = hasher.Write([]byte{labelsSeparator})
	}
	return hasher.Sum64()
}
