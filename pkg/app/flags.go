package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// BindFlags registers all operator CLI flags on cmd, using the current cfg
// values (already merged with env vars and hardcoded defaults) as flag defaults.
// No env-var lookups are performed here; environment variables are handled
// exclusively by ParseEnv. An explicit CLI flag always wins.
//
// The returned func must be called after cobra.Execute() to apply []string
// slice overrides that cannot be handled with a simple default binding.
func BindFlags(cfg *Config, rootCmd *cobra.Command, cmd *cobra.Command) func() {
	bindAppFlags(cfg, cmd)
	bindKubeFlags(cfg, cmd)
	bindLogFlags(cfg, cmd)
	applyAdmission := bindAdmissionWebhookFlags(cfg, cmd)
	applyConversion := bindConversionWebhookFlags(cfg, cmd)
	bindDebugFlags(cfg, rootCmd, cmd)

	return func() {
		applyAdmission()
		applyConversion()
	}
}

func bindAppFlags(cfg *Config, cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&cfg.App.HooksDir, "hooks-dir", cfg.App.HooksDir, "A path to a hooks file structure. Can be set with $SHELL_OPERATOR_HOOKS_DIR.")
	f.StringVar(&cfg.App.TempDir, "tmp-dir", cfg.App.TempDir, "A path to store temporary files with data for hooks. Can be set with $SHELL_OPERATOR_TMP_DIR.")
	f.StringVar(&cfg.App.ListenAddress, "listen-address", cfg.App.ListenAddress, "Address to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_ADDRESS.")
	f.StringVar(&cfg.App.ListenPort, "listen-port", cfg.App.ListenPort, "Port to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_PORT.")
	f.StringVar(&cfg.App.PrometheusMetricsPrefix, "prometheus-metrics-prefix", cfg.App.PrometheusMetricsPrefix, "Prefix for Prometheus metrics. Can be set with $SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX.")
	f.StringVar(&cfg.App.Namespace, "namespace", cfg.App.Namespace, "A namespace of a shell-operator. Used to set up validating webhooks. Can be set with $SHELL_OPERATOR_NAMESPACE.")
}

func bindKubeFlags(cfg *Config, cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&cfg.Kube.Context, "kube-context", cfg.Kube.Context, "The name of the kubeconfig context to use. Can be set with $KUBE_CONTEXT.")
	f.StringVar(&cfg.Kube.Config, "kube-config", cfg.Kube.Config, "Path to the kubeconfig file. Can be set with $KUBE_CONFIG.")
	f.StringVar(&cfg.Kube.Server, "kube-server", cfg.Kube.Server, "The address and port of the Kubernetes API server. Can be set with $KUBE_SERVER.")
	f.Float32Var(&cfg.Kube.ClientQPS, "kube-client-qps", cfg.Kube.ClientQPS, "QPS for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_QPS.")
	f.IntVar(&cfg.Kube.ClientBurst, "kube-client-burst", cfg.Kube.ClientBurst, "Burst for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_BURST.")
	f.Float32Var(&cfg.ObjectPatcher.KubeClientQPS, "object-patcher-kube-client-qps", cfg.ObjectPatcher.KubeClientQPS, "QPS for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_QPS.")
	f.IntVar(&cfg.ObjectPatcher.KubeClientBurst, "object-patcher-kube-client-burst", cfg.ObjectPatcher.KubeClientBurst, "Burst for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_BURST.")
	f.DurationVar(&cfg.ObjectPatcher.KubeClientTimeout, "object-patcher-kube-client-timeout", cfg.ObjectPatcher.KubeClientTimeout, "Timeout for object patcher requests to the Kubernetes API server. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT.")
}

// bindAdmissionWebhookFlags registers validating-webhook flags and returns a fixup
// function that applies the CLI []string ClientCA override after flag parsing.
func bindAdmissionWebhookFlags(cfg *Config, cmd *cobra.Command) func() {
	f := cmd.Flags()
	f.StringVar(&cfg.Admission.ConfigurationName, "validating-webhook-configuration-name", cfg.Admission.ConfigurationName, "A name of a ValidatingWebhookConfiguration resource. Can be set with $VALIDATING_WEBHOOK_CONFIGURATION_NAME.")
	f.StringVar(&cfg.Admission.ServiceName, "validating-webhook-service-name", cfg.Admission.ServiceName, "A name of a service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.")
	f.StringVar(&cfg.Admission.ServerCert, "validating-webhook-server-cert", cfg.Admission.ServerCert, "A path to a server certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.")
	f.StringVar(&cfg.Admission.ServerKey, "validating-webhook-server-key", cfg.Admission.ServerKey, "A path to a server private key for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.")
	f.StringVar(&cfg.Admission.CA, "validating-webhook-ca", cfg.Admission.CA, "A path to a CA certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CA.")

	// ClientCA is a []string; StringArrayVar stores each flag invocation separately,
	// preserving the env value when no CLI flag is given.
	envClientCA := cfg.Admission.ClientCA
	var cliClientCA []string
	f.StringArrayVar(&cliClientCA, "validating-webhook-client-ca", nil, "Paths to client CA certificates for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.")

	f.StringVar(&cfg.Admission.FailurePolicy, "validating-webhook-failure-policy", cfg.Admission.FailurePolicy, "Default FailurePolicy for ValidatingWebhookConfiguration (Fail or Ignore). Can be set with $VALIDATING_WEBHOOK_FAILURE_POLICY.")
	f.StringVar(&cfg.Admission.ListenPort, "validating-webhook-listen-port", cfg.Admission.ListenPort, "Listen port for ValidatingWebhookConfiguration server. Can be set with $VALIDATING_WEBHOOK_LISTEN_PORT.")
	f.StringVar(&cfg.Admission.ListenAddress, "validating-webhook-listen-address", cfg.Admission.ListenAddress, "Listen address for ValidatingWebhookConfiguration server. Can be set with $VALIDATING_WEBHOOK_LISTEN_ADDRESS.")

	return func() {
		if len(cliClientCA) > 0 {
			cfg.Admission.ClientCA = cliClientCA
		} else {
			cfg.Admission.ClientCA = envClientCA
		}
	}
}

// bindConversionWebhookFlags registers conversion-webhook flags and returns a fixup
// function that applies the CLI []string ClientCA override after flag parsing.
func bindConversionWebhookFlags(cfg *Config, cmd *cobra.Command) func() {
	f := cmd.Flags()
	f.StringVar(&cfg.Conversion.ServiceName, "conversion-webhook-service-name", cfg.Conversion.ServiceName, "A name of a service for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVICE_NAME.")
	f.StringVar(&cfg.Conversion.ServerCert, "conversion-webhook-server-cert", cfg.Conversion.ServerCert, "A path to a server certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_CERT.")
	f.StringVar(&cfg.Conversion.ServerKey, "conversion-webhook-server-key", cfg.Conversion.ServerKey, "A path to a server private key for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_KEY.")
	f.StringVar(&cfg.Conversion.CA, "conversion-webhook-ca", cfg.Conversion.CA, "A path to a CA certificate for conversion webhook. Can be set with $CONVERSION_WEBHOOK_CA.")

	envClientCA := cfg.Conversion.ClientCA
	var cliClientCA []string
	f.StringArrayVar(&cliClientCA, "conversion-webhook-client-ca", nil, "Paths to client CA certificates for conversion webhook. Can be set with $CONVERSION_WEBHOOK_CLIENT_CA.")

	f.StringVar(&cfg.Conversion.ListenPort, "conversion-webhook-listen-port", cfg.Conversion.ListenPort, "Listen port for conversion webhook server. Can be set with $CONVERSION_WEBHOOK_LISTEN_PORT.")
	f.StringVar(&cfg.Conversion.ListenAddress, "conversion-webhook-listen-address", cfg.Conversion.ListenAddress, "Listen address for conversion webhook server. Can be set with $CONVERSION_WEBHOOK_LISTEN_ADDRESS.")

	return func() {
		if len(cliClientCA) > 0 {
			cfg.Conversion.ClientCA = cliClientCA
		} else {
			cfg.Conversion.ClientCA = envClientCA
		}
	}
}

func bindLogFlags(cfg *Config, cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&cfg.Log.Level, "log-level", cfg.Log.Level, "Logging level: debug, info, error. Default is info. Can be set with $LOG_LEVEL.")
	f.StringVar(&cfg.Log.Type, "log-type", cfg.Log.Type, "Logging formatter type: json, text or color. Default is text. Can be set with $LOG_TYPE.")
	f.BoolVar(&cfg.Log.NoTime, "log-no-time", cfg.Log.NoTime, "Disable timestamp logging. Can be set with $LOG_NO_TIME.")
	f.BoolVar(&cfg.Log.ProxyHookJSON, "log-proxy-hook-json", cfg.Log.ProxyHookJSON, "Proxy hook stdout/stderr JSON logging. Can be set with $LOG_PROXY_HOOK_JSON.")
}

func bindDebugFlags(cfg *Config, rootCmd *cobra.Command, cmd *cobra.Command) {
	// Sync the package-level global so debug sub-commands (queue, hook, etc.)
	// that bind to DebugUnixSocket get the env/default value.
	DebugUnixSocket = cfg.Debug.UnixSocket

	f := cmd.Flags()
	f.StringVar(&cfg.Debug.UnixSocket, "debug-unix-socket", cfg.Debug.UnixSocket, "A path to a unix socket for a debug endpoint. Can be set with $DEBUG_UNIX_SOCKET.")
	_ = f.MarkHidden("debug-unix-socket")

	f.StringVar(&cfg.Debug.HTTPServerAddr, "debug-http-addr", cfg.Debug.HTTPServerAddr, "HTTP address for a debug endpoint. Can be set with $DEBUG_HTTP_SERVER_ADDR.")
	_ = f.MarkHidden("debug-http-addr")

	f.BoolVar(&cfg.Debug.KeepTempFiles, "debug-keep-tmp-files", cfg.Debug.KeepTempFiles, "Set to true to disable cleanup of temporary files. Can be set with $DEBUG_KEEP_TMP_FILES.")
	_ = f.MarkHidden("debug-keep-tmp-files")

	f.BoolVar(&cfg.Debug.KubernetesAPI, "debug-kubernetes-api", cfg.Debug.KubernetesAPI, "Enable client-go debug messages. Can be set with $DEBUG_KUBERNETES_API.")
	_ = f.MarkHidden("debug-kubernetes-api")

	// debug-options prints all hidden debug-* flags of the start command.
	startCmd := cmd
	debugOptionsCmd := &cobra.Command{
		Use:    "debug-options",
		Short:  "Show help for debug flags of the start command.",
		Hidden: true,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(os.Stdout, "usage: %s start [flags]\n\nDebug flags:\n", rootCmd.Use)
			startCmd.Flags().VisitAll(func(fl *pflag.Flag) {
				if fl.Hidden && strings.HasPrefix(fl.Name, "debug-") {
					fmt.Fprintf(os.Stdout, "  --%s\n        %s\n", fl.Name, fl.Usage)
				}
			})
			os.Exit(0)
		},
	}
	rootCmd.AddCommand(debugOptionsCmd)
}
