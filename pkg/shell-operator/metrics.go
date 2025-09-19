package shell_operator

import (
	"fmt"
	"net/http"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/internal/metrics"
	"github.com/flant/shell-operator/pkg/app"
)

// setupMetricStorage creates and initializes metrics storage for built-in operator metrics.
// If MetricStorage is already set via options, it uses that; otherwise creates a new one.
func (op *ShellOperator) setupMetricStorage() error {
	// Use provided metric storage or create default
	if op.MetricStorage == nil {
		op.MetricStorage = metricsstorage.NewMetricStorage(
			metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
			metricsstorage.WithLogger(op.logger.Named("metric-storage")),
		)
	}

	if err := metrics.RegisterCommonMetrics(op.MetricStorage); err != nil {
		return fmt.Errorf("register common metrics: %w", err)
	}

	if err := metrics.RegisterTaskQueueMetrics(op.MetricStorage); err != nil {
		return fmt.Errorf("register task queue metrics: %w", err)
	}

	if err := metrics.RegisterKubeEventsManagerMetrics(op.MetricStorage, []string{"hook", "binding", "queue"}); err != nil {
		return fmt.Errorf("register kube events manager metrics: %w", err)
	}

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", op.MetricStorage.Handler().ServeHTTP)

	return nil
}

// setupHookMetricStorage creates and initializes metrics storage for hook metrics.
// If HookMetricStorage is already set via options, it uses that; otherwise creates a new one.
func (op *ShellOperator) setupHookMetricStorage() {
	// Use provided hook metric storage or create default
	if op.HookMetricStorage == nil {
		op.HookMetricStorage = metricsstorage.NewMetricStorage(
			metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
			metricsstorage.WithNewRegistry(),
			metricsstorage.WithLogger(op.logger.Named("hook-metric-storage")),
		)
	}

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics/hooks", op.HookMetricStorage.Handler().ServeHTTP)
}
