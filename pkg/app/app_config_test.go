package app

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindAndParse creates a fresh cobra root+start command pair, registers all
// flags via BindFlags, parses argv, and calls the returned post-parse fixup.
// cfg is ready to inspect on return.
func bindAndParse(t *testing.T, cfg *Config, argv []string) {
	t.Helper()
	rootCmd := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true}
	startCmd := &cobra.Command{
		Use:  "start",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	rootCmd.AddCommand(startCmd)
	applySlices := BindFlags(cfg, rootCmd, startCmd)
	rootCmd.SetArgs(append([]string{"start"}, argv...))
	require.NoError(t, rootCmd.Execute())
	applySlices()
}

// ---------------------------------------------------------------------------
// 1. All flags fill configuration parameters
// ---------------------------------------------------------------------------

func TestBindFlags_AllFlagsSetConfigParams(t *testing.T) {
	cfg := NewConfig()

	bindAndParse(t, cfg, []string{
		// app flags
		"--hooks-dir=/test/hooks",
		"--tmp-dir=/test/tmp",
		"--listen-address=127.0.0.1",
		"--listen-port=9999",
		"--prometheus-metrics-prefix=my_prefix_",
		"--namespace=mynamespace",
		// kube flags
		"--kube-context=my-context",
		"--kube-config=/home/.kube/config",
		"--kube-server=https://kube-api:6443",
		"--kube-client-qps=20",
		"--kube-client-burst=40",
		// object patcher flags
		"--object-patcher-kube-client-qps=15",
		"--object-patcher-kube-client-burst=30",
		"--object-patcher-kube-client-timeout=20s",
		// validating webhook flags
		"--validating-webhook-configuration-name=my-config",
		"--validating-webhook-service-name=my-svc",
		"--validating-webhook-server-cert=/certs/tls.crt",
		"--validating-webhook-server-key=/certs/tls.key",
		"--validating-webhook-ca=/certs/ca.crt",
		"--validating-webhook-client-ca=/certs/client-ca.crt",
		"--validating-webhook-failure-policy=Ignore",
		"--validating-webhook-listen-port=9090",
		"--validating-webhook-listen-address=0.0.0.0",
		// conversion webhook flags
		"--conversion-webhook-service-name=conv-svc",
		"--conversion-webhook-server-cert=/conv-certs/tls.crt",
		"--conversion-webhook-server-key=/conv-certs/tls.key",
		"--conversion-webhook-ca=/conv-certs/ca.crt",
		"--conversion-webhook-client-ca=/conv-certs/client-ca.crt",
		"--conversion-webhook-listen-port=9091",
		"--conversion-webhook-listen-address=0.0.0.0",
		// log flags
		"--log-level=debug",
		"--log-type=json",
		"--log-no-time",
		"--log-proxy-hook-json",
		// debug flags
		"--debug-http-addr=127.0.0.1:9650",
		"--debug-keep-tmp-files",
		"--debug-kubernetes-api",
		"--debug-unix-socket=/var/run/shell-operator/debug-test.socket",
	})

	// App
	assert.Equal(t, "/test/hooks", cfg.App.HooksDir)
	assert.Equal(t, "/test/tmp", cfg.App.TempDir)
	assert.Equal(t, "127.0.0.1", cfg.App.ListenAddress)
	assert.Equal(t, "9999", cfg.App.ListenPort)
	assert.Equal(t, "my_prefix_", cfg.App.PrometheusMetricsPrefix)
	assert.Equal(t, "mynamespace", cfg.App.Namespace)

	// Kube
	assert.Equal(t, "my-context", cfg.Kube.Context)
	assert.Equal(t, "/home/.kube/config", cfg.Kube.Config)
	assert.Equal(t, "https://kube-api:6443", cfg.Kube.Server)
	assert.InDelta(t, float32(20), cfg.Kube.ClientQPS, 0.001)
	assert.Equal(t, 40, cfg.Kube.ClientBurst)

	// ObjectPatcher
	assert.InDelta(t, float32(15), cfg.ObjectPatcher.KubeClientQPS, 0.001)
	assert.Equal(t, 30, cfg.ObjectPatcher.KubeClientBurst)
	assert.Equal(t, 20*time.Second, cfg.ObjectPatcher.KubeClientTimeout)

	// Admission (validating webhook)
	assert.Equal(t, "my-config", cfg.Admission.ConfigurationName)
	assert.Equal(t, "my-svc", cfg.Admission.ServiceName)
	assert.Equal(t, "/certs/tls.crt", cfg.Admission.ServerCert)
	assert.Equal(t, "/certs/tls.key", cfg.Admission.ServerKey)
	assert.Equal(t, "/certs/ca.crt", cfg.Admission.CA)
	assert.Equal(t, []string{"/certs/client-ca.crt"}, cfg.Admission.ClientCA)
	assert.Equal(t, "Ignore", cfg.Admission.FailurePolicy)
	assert.Equal(t, "9090", cfg.Admission.ListenPort)
	assert.Equal(t, "0.0.0.0", cfg.Admission.ListenAddress)

	// Conversion webhook
	assert.Equal(t, "conv-svc", cfg.Conversion.ServiceName)
	assert.Equal(t, "/conv-certs/tls.crt", cfg.Conversion.ServerCert)
	assert.Equal(t, "/conv-certs/tls.key", cfg.Conversion.ServerKey)
	assert.Equal(t, "/conv-certs/ca.crt", cfg.Conversion.CA)
	assert.Equal(t, []string{"/conv-certs/client-ca.crt"}, cfg.Conversion.ClientCA)
	assert.Equal(t, "9091", cfg.Conversion.ListenPort)
	assert.Equal(t, "0.0.0.0", cfg.Conversion.ListenAddress)

	// Log
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Type)
	assert.True(t, cfg.Log.NoTime)
	assert.True(t, cfg.Log.ProxyHookJSON)

	// Debug
	assert.Equal(t, "127.0.0.1:9650", cfg.Debug.HTTPServerAddr)
	assert.True(t, cfg.Debug.KeepTempFiles)
	assert.True(t, cfg.Debug.KubernetesAPI)
	assert.Equal(t, "/var/run/shell-operator/debug-test.socket", cfg.Debug.UnixSocket)
}

// ---------------------------------------------------------------------------
// 2. All envs fill configuration parameters
// ---------------------------------------------------------------------------

func TestParseEnv_AllEnvsSetConfigParams(t *testing.T) {
	t.Setenv("SHELL_OPERATOR_HOOKS_DIR", "/env/hooks")
	t.Setenv("SHELL_OPERATOR_TMP_DIR", "/env/tmp")
	t.Setenv("SHELL_OPERATOR_LISTEN_ADDRESS", "10.0.0.1")
	t.Setenv("SHELL_OPERATOR_LISTEN_PORT", "8888")
	t.Setenv("SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX", "env_prefix_")
	t.Setenv("SHELL_OPERATOR_NAMESPACE", "env-namespace")

	t.Setenv("KUBE_CONTEXT", "env-context")
	t.Setenv("KUBE_CONFIG", "/env/.kube/config")
	t.Setenv("KUBE_SERVER", "https://env-kube:6443")
	t.Setenv("KUBE_CLIENT_QPS", "25")
	t.Setenv("KUBE_CLIENT_BURST", "50")

	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_QPS", "12")
	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_BURST", "24")
	t.Setenv("OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT", "30s")

	t.Setenv("VALIDATING_WEBHOOK_CONFIGURATION_NAME", "env-config")
	t.Setenv("VALIDATING_WEBHOOK_SERVICE_NAME", "env-svc")
	t.Setenv("VALIDATING_WEBHOOK_SERVER_CERT", "/env-certs/tls.crt")
	t.Setenv("VALIDATING_WEBHOOK_SERVER_KEY", "/env-certs/tls.key")
	t.Setenv("VALIDATING_WEBHOOK_CA", "/env-certs/ca.crt")
	t.Setenv("VALIDATING_WEBHOOK_CLIENT_CA", "/env-certs/client-ca.crt")
	t.Setenv("VALIDATING_WEBHOOK_FAILURE_POLICY", "Ignore")
	t.Setenv("VALIDATING_WEBHOOK_LISTEN_PORT", "9181")
	t.Setenv("VALIDATING_WEBHOOK_LISTEN_ADDRESS", "1.2.3.4")

	t.Setenv("CONVERSION_WEBHOOK_SERVICE_NAME", "env-conv-svc")
	t.Setenv("CONVERSION_WEBHOOK_SERVER_CERT", "/env-conv-certs/tls.crt")
	t.Setenv("CONVERSION_WEBHOOK_SERVER_KEY", "/env-conv-certs/tls.key")
	t.Setenv("CONVERSION_WEBHOOK_CA", "/env-conv-certs/ca.crt")
	t.Setenv("CONVERSION_WEBHOOK_CLIENT_CA", "/env-conv-certs/client-ca.crt")
	t.Setenv("CONVERSION_WEBHOOK_LISTEN_PORT", "9182")
	t.Setenv("CONVERSION_WEBHOOK_LISTEN_ADDRESS", "5.6.7.8")

	t.Setenv("DEBUG_HTTP_SERVER_ADDR", "127.0.0.1:9651")
	t.Setenv("DEBUG_KEEP_TMP_FILES", "true")
	t.Setenv("DEBUG_KUBERNETES_API", "true")
	t.Setenv("DEBUG_UNIX_SOCKET", "/var/run/shell-operator/env-debug.socket")

	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("LOG_TYPE", "json")
	t.Setenv("LOG_NO_TIME", "true")
	t.Setenv("LOG_PROXY_HOOK_JSON", "true")

	cfg := NewConfig()
	require.NoError(t, ParseEnv(cfg))

	// App
	assert.Equal(t, "/env/hooks", cfg.App.HooksDir)
	assert.Equal(t, "/env/tmp", cfg.App.TempDir)
	assert.Equal(t, "10.0.0.1", cfg.App.ListenAddress)
	assert.Equal(t, "8888", cfg.App.ListenPort)
	assert.Equal(t, "env_prefix_", cfg.App.PrometheusMetricsPrefix)
	assert.Equal(t, "env-namespace", cfg.App.Namespace)

	// Kube
	assert.Equal(t, "env-context", cfg.Kube.Context)
	assert.Equal(t, "/env/.kube/config", cfg.Kube.Config)
	assert.Equal(t, "https://env-kube:6443", cfg.Kube.Server)
	assert.InDelta(t, float32(25), cfg.Kube.ClientQPS, 0.001)
	assert.Equal(t, 50, cfg.Kube.ClientBurst)

	// ObjectPatcher
	assert.InDelta(t, float32(12), cfg.ObjectPatcher.KubeClientQPS, 0.001)
	assert.Equal(t, 24, cfg.ObjectPatcher.KubeClientBurst)
	assert.Equal(t, 30*time.Second, cfg.ObjectPatcher.KubeClientTimeout)

	// Admission
	assert.Equal(t, "env-config", cfg.Admission.ConfigurationName)
	assert.Equal(t, "env-svc", cfg.Admission.ServiceName)
	assert.Equal(t, "/env-certs/tls.crt", cfg.Admission.ServerCert)
	assert.Equal(t, "/env-certs/tls.key", cfg.Admission.ServerKey)
	assert.Equal(t, "/env-certs/ca.crt", cfg.Admission.CA)
	assert.Equal(t, []string{"/env-certs/client-ca.crt"}, cfg.Admission.ClientCA)
	assert.Equal(t, "Ignore", cfg.Admission.FailurePolicy)
	assert.Equal(t, "9181", cfg.Admission.ListenPort)
	assert.Equal(t, "1.2.3.4", cfg.Admission.ListenAddress)

	// Conversion
	assert.Equal(t, "env-conv-svc", cfg.Conversion.ServiceName)
	assert.Equal(t, "/env-conv-certs/tls.crt", cfg.Conversion.ServerCert)
	assert.Equal(t, "/env-conv-certs/tls.key", cfg.Conversion.ServerKey)
	assert.Equal(t, "/env-conv-certs/ca.crt", cfg.Conversion.CA)
	assert.Equal(t, []string{"/env-conv-certs/client-ca.crt"}, cfg.Conversion.ClientCA)
	assert.Equal(t, "9182", cfg.Conversion.ListenPort)
	assert.Equal(t, "5.6.7.8", cfg.Conversion.ListenAddress)

	// Debug
	assert.Equal(t, "127.0.0.1:9651", cfg.Debug.HTTPServerAddr)
	assert.True(t, cfg.Debug.KeepTempFiles)
	assert.True(t, cfg.Debug.KubernetesAPI)
	assert.Equal(t, "/var/run/shell-operator/env-debug.socket", cfg.Debug.UnixSocket)

	// Log
	assert.Equal(t, "error", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Type)
	assert.True(t, cfg.Log.NoTime)
	assert.True(t, cfg.Log.ProxyHookJSON)
}

// ---------------------------------------------------------------------------
// 3. Envs override flags when both are provided
//
// The priority chain is: CLI flag (explicit) > env var > hardcoded default.
// BindFlags uses the current cfg values (already populated by ParseEnv) as
// flag defaults. A flag that is NOT explicitly passed on the CLI therefore
// retains the env value, so env vars effectively override hardcoded defaults
// and any flag that is not present on the command line.
// ---------------------------------------------------------------------------

func TestEnvOverridesDefaultWhenFlagNotPassed(t *testing.T) {
	// Set env vars that differ from hardcoded defaults.
	t.Setenv("SHELL_OPERATOR_HOOKS_DIR", "/env-override/hooks")
	t.Setenv("SHELL_OPERATOR_LISTEN_PORT", "7777")
	t.Setenv("KUBE_CONTEXT", "override-context")
	t.Setenv("KUBE_CLIENT_QPS", "99")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DEBUG_KEEP_TMP_FILES", "true")
	t.Setenv("VALIDATING_WEBHOOK_LISTEN_PORT", "9999")
	t.Setenv("CONVERSION_WEBHOOK_LISTEN_PORT", "9998")

	cfg := NewConfig()
	require.NoError(t, ParseEnv(cfg))

	// BindFlags with no explicit CLI args; env values must be preserved.
	bindAndParse(t, cfg, []string{})

	assert.Equal(t, "/env-override/hooks", cfg.App.HooksDir)
	assert.Equal(t, "7777", cfg.App.ListenPort)
	assert.Equal(t, "override-context", cfg.Kube.Context)
	assert.InDelta(t, float32(99), cfg.Kube.ClientQPS, 0.001)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.True(t, cfg.Debug.KeepTempFiles)
	assert.Equal(t, "9999", cfg.Admission.ListenPort)
	assert.Equal(t, "9998", cfg.Conversion.ListenPort)
}

// TestCLIFlagOverridesEnv verifies that an explicit CLI flag wins over an env var.
// This is the complementary case to TestEnvOverridesDefaultWhenFlagNotPassed.
func TestCLIFlagOverridesEnv(t *testing.T) {
	// Set env vars.
	t.Setenv("SHELL_OPERATOR_HOOKS_DIR", "/env/hooks")
	t.Setenv("SHELL_OPERATOR_LISTEN_PORT", "7777")
	t.Setenv("LOG_LEVEL", "error")

	cfg := NewConfig()
	require.NoError(t, ParseEnv(cfg))

	// Explicitly pass CLI flags that conflict with the env values.
	bindAndParse(t, cfg, []string{
		"--hooks-dir=/cli/hooks",
		"--listen-port=1234",
		"--log-level=debug",
	})

	// CLI must win.
	assert.Equal(t, "/cli/hooks", cfg.App.HooksDir)
	assert.Equal(t, "1234", cfg.App.ListenPort)
	assert.Equal(t, "debug", cfg.Log.Level)
}
