package shell_operator

import (
	"context"
	"net/http"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

// defaultHookMetricStorage creates MetricStorage object
// with new registry to scrape hook metrics on separate port.
func defaultHookMetricStorage(ctx context.Context) *metric_storage.MetricStorage {
	metricStorage := metric_storage.NewMetricStorage(ctx, app.PrometheusMetricsPrefix, true)
	return metricStorage
}

func setupHookMetricStorageAndServer(ctx context.Context) (*metric_storage.MetricStorage, error) {
	if app.HookMetricsListenPort == "" || app.HookMetricsListenPort == app.ListenPort {
		// No separate metric storage required.
		return nil, nil
	}
	// create new metric storage for hooks
	metricStorage := defaultHookMetricStorage(ctx)
	// Create new ServeMux, and serve on custom port.
	mux := http.NewServeMux()
	err := startHttpServer(app.ListenAddress, app.HookMetricsListenPort, mux)
	if err != nil {
		return nil, err
	}
	// register scrape handler
	mux.Handle("/metrics", metricStorage.Handler())
	return metricStorage, nil
}
