package shell_operator

import (
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
)

var DefaultMainKubeClientMetricLabels = map[string]string{"component": "main"}
var DefaultObjectPatcherKubeClientMetricLabels = map[string]string{"component": "object_patcher"}

func (op *ShellOperator) GetMainKubeClientMetricLabels() map[string]string {
	if op.MainKubeClientMetricLabels == nil {
		return DefaultMainKubeClientMetricLabels
	}
	return op.MainKubeClientMetricLabels
}

func (op *ShellOperator) GetObjectPatcherKubeClientMetricLabels() map[string]string {
	if op.MainKubeClientMetricLabels == nil {
		return DefaultObjectPatcherKubeClientMetricLabels
	}
	return op.ObjectPatcherKubeClientMetricLabels
}

// InitMainKubeClient initializes a Kubernetes client for hooks. No timeout specified, because
// timeout will reset connections for Watchers.
func (op *ShellOperator) InitMainKubeClient() (kube.KubernetesClient, error) {
	client := kube.NewKubernetesClient()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.KubeClientQps, app.KubeClientBurst)
	client.WithMetricStorage(op.MetricStorage)
	client.WithMetricLabels(op.GetMainKubeClientMetricLabels())
	if err := client.Init(); err != nil {
		return nil, err
	}
	return client, nil
}

// InitObjectPatcherKubeClient initializes a Kubernetes client for ObjectPatcher. Timeout is specified here.
func (op *ShellOperator) InitObjectPatcherKubeClient() (kube.KubernetesClient, error) {
	client := kube.NewKubernetesClient()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.ObjectPatcherKubeClientQps, app.ObjectPatcherKubeClientBurst)
	client.WithTimeout(app.ObjectPatcherKubeClientTimeout)
	client.WithMetricStorage(op.MetricStorage)
	client.WithMetricLabels(op.GetObjectPatcherKubeClientMetricLabels())
	if err := client.Init(); err != nil {
		return nil, err
	}
	return client, nil
}
