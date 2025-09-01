package metric_test

import (
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metric_storage/vault"
)

var (
	_ metric.Storage = (metric.Storage)(nil)
	_ metric.Storage = (*metric.StorageMock)(nil)

	_ metric.GroupedStorage = (*vault.GroupedVault)(nil)
	_ metric.GroupedStorage = (*metric.GroupedStorageMock)(nil)
)
