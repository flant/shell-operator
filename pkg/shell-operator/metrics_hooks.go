package shell_operator

import (
	"context"
	"net/http"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

// DefaultHookMetricStorage creates MetricStorage object
// with new registry to scrape hook metrics on separate port.
func DefaultHookMetricStorage(ctx context.Context) *metric_storage.MetricStorage {
	metricStorage := metric_storage.NewMetricStorage()
	metricStorage.WithContext(ctx)
	metricStorage.WithPrefix(app.PrometheusMetricsPrefix)
	metricStorage.WithNewRegistry()
	metricStorage.Start()
	return metricStorage
}

func SetupHookMetricStorageAndServer(ctx context.Context) (*metric_storage.MetricStorage, error) {
	if app.HookMetricsListenPort == "" || app.HookMetricsListenPort == app.ListenPort {
		// No separate metric storage required.
		return nil, nil
	}
	// create new metric storage for hooks
	metricStorage := DefaultHookMetricStorage(ctx)
	// Create new ServeMux, and serve on custom port.
	mux := http.NewServeMux()
	err := StartHttpServer(app.ListenAddress, app.HookMetricsListenPort, mux)
	if err != nil {
		return nil, err
	}
	// register scrape handler
	mux.Handle("/metrics", metricStorage.Handler())
	return metricStorage, nil
}
