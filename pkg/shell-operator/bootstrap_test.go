package shell_operator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/app"
)

// TestKubeClientConfigsFromAppConfig_DerivedFromConfig is a basic check that
// the per-client KubeClientConfigs are built straight from the supplied
// *app.Config (including the special "object_patcher_" metric prefix for the
// patcher client).
func TestKubeClientConfigsFromAppConfig_DerivedFromConfig(t *testing.T) {
	cfg := &app.Config{
		App: app.AppSettings{
			ListenAddress:           "127.0.0.1",
			ListenPort:              "9000",
			PrometheusMetricsPrefix: "embedded_",
		},
		Kube: app.KubeSettings{
			Context:     "explicit-ctx",
			Config:      "/explicit/kubeconfig",
			ClientQPS:   42,
			ClientBurst: 84,
		},
		ObjectPatcher: app.ObjectPatcherSettings{
			KubeClientQPS:     11,
			KubeClientBurst:   22,
			KubeClientTimeout: 7 * time.Second,
		},
	}

	addr, port := listenAddrFromAppConfig(cfg)
	main, patcher := kubeClientConfigsFromAppConfig(cfg)

	assert.Equal(t, "127.0.0.1", addr)
	assert.Equal(t, "9000", port)

	assert.Equal(t, "explicit-ctx", main.Context)
	assert.Equal(t, "/explicit/kubeconfig", main.Config)
	assert.InDelta(t, float32(42), main.QPS, 0.001)
	assert.Equal(t, 84, main.Burst)
	assert.Equal(t, time.Duration(0), main.Timeout)
	assert.Equal(t, "embedded_", main.MetricPrefix)

	assert.Equal(t, "explicit-ctx", patcher.Context)
	assert.Equal(t, "/explicit/kubeconfig", patcher.Config)
	assert.InDelta(t, float32(11), patcher.QPS, 0.001)
	assert.Equal(t, 22, patcher.Burst)
	assert.Equal(t, 7*time.Second, patcher.Timeout)
	assert.Equal(t, "object_patcher_", patcher.MetricPrefix)
}

// TestKubeClientConfigsFromAppConfig_NilCfg verifies the nil-cfg fallback
// yields zero KubeClientConfig values (which klient treats as in-cluster
// defaults) without consulting the environment.
func TestKubeClientConfigsFromAppConfig_NilCfg(t *testing.T) {
	t.Setenv("KUBE_CONTEXT", "env-ctx")
	t.Setenv("SHELL_OPERATOR_LISTEN_PORT", "7777")

	addr, port := listenAddrFromAppConfig(nil)
	main, patcher := kubeClientConfigsFromAppConfig(nil)

	assert.Empty(t, addr)
	assert.Empty(t, port)
	assert.Equal(t, KubeClientConfig{}, main)
	assert.Equal(t, KubeClientConfig{}, patcher)
}

// TestAssembleFromConfig_EnvDoesNotOverrideConfig is the regression test that
// guards the library contract: when the consumer hands us an *app.Config,
// environment variables MUST NOT silently override its values. We set env
// vars that conflict with every field used by the assembly path and verify
// the derived listen address, KubeClient settings and metric prefix all come
// from cfg instead.
func TestAssembleFromConfig_EnvDoesNotOverrideConfig(t *testing.T) {
	// Set env vars that, if mistakenly consulted, would change every
	// derived value.
	t.Setenv("SHELL_OPERATOR_LISTEN_ADDRESS", "9.9.9.9")
	t.Setenv("SHELL_OPERATOR_LISTEN_PORT", "9999")
	t.Setenv("SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX", "env_prefix_")
	t.Setenv("KUBE_CONTEXT", "env-ctx")
	t.Setenv("KUBE_CONFIG", "/env/kubeconfig")
	t.Setenv("KUBE_CLIENT_QPS", "999")
	t.Setenv("KUBE_CLIENT_BURST", "888")
	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_QPS", "777")
	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_BURST", "666")
	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT", "55s")

	cfg := &app.Config{
		App: app.AppSettings{
			ListenAddress:           "127.0.0.1",
			ListenPort:              "9000",
			PrometheusMetricsPrefix: "lib_prefix_",
		},
		Kube: app.KubeSettings{
			Context:     "lib-ctx",
			Config:      "/lib/kubeconfig",
			ClientQPS:   1,
			ClientBurst: 2,
		},
		ObjectPatcher: app.ObjectPatcherSettings{
			KubeClientQPS:     3,
			KubeClientBurst:   4,
			KubeClientTimeout: 5 * time.Second,
		},
	}

	addr, port := listenAddrFromAppConfig(cfg)
	main, patcher := kubeClientConfigsFromAppConfig(cfg)

	// Listen address/port must come from cfg, not env.
	assert.Equal(t, "127.0.0.1", addr)
	assert.Equal(t, "9000", port)

	// Main client values come from cfg.Kube / cfg.App.
	assert.Equal(t, "lib-ctx", main.Context)
	assert.Equal(t, "/lib/kubeconfig", main.Config)
	assert.InDelta(t, float32(1), main.QPS, 0.001)
	assert.Equal(t, 2, main.Burst)
	assert.Equal(t, "lib_prefix_", main.MetricPrefix)

	// Patcher client values come from cfg.ObjectPatcher.
	assert.Equal(t, "lib-ctx", patcher.Context)
	assert.Equal(t, "/lib/kubeconfig", patcher.Config)
	assert.InDelta(t, float32(3), patcher.QPS, 0.001)
	assert.Equal(t, 4, patcher.Burst)
	assert.Equal(t, 5*time.Second, patcher.Timeout)
	assert.Equal(t, "object_patcher_", patcher.MetricPrefix)

	// And the source-of-truth cfg itself is untouched by the call.
	assert.Equal(t, "127.0.0.1", cfg.App.ListenAddress)
	assert.Equal(t, "9000", cfg.App.ListenPort)
	assert.Equal(t, "lib-ctx", cfg.Kube.Context)
	assert.InDelta(t, float32(1), cfg.Kube.ClientQPS, 0.001)
}
