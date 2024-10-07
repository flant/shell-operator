package metric_test

import (
	"github.com/flant/shell-operator/pkg/metric"
)

var (
	_ metric.ConstCollector = (*metric.ConstCounterCollector)(nil)
	_ metric.ConstCollector = (*metric.ConstGaugeCollector)(nil)
)
