package shell_operator

import (
	"net/http"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

func (op *ShellOperator) setupHookMetricStorage() {
	metricStorage := metric_storage.NewMetricStorage(op.ctx, app.PrometheusMetricsPrefix, true)

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics/hooks", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.MetricStorage = metricStorage
}
