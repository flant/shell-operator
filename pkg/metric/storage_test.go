package metric_test

import (
	"github.com/flant/shell-operator/pkg/metric"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/flant/shell-operator/pkg/metric_storage/vault"
)

var (
	_ metric.Storage = (*metricstorage.MetricStorage)(nil)
	_ metric.Storage = (*metric.StorageMock)(nil)

	_ metric.GroupedStorage = (*vault.GroupedVault)(nil)
	_ metric.GroupedStorage = (*metric.GroupedStorageMock)(nil)
)
