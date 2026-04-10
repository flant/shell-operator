package app

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

// BindFlags registers all operator CLI flags on cmd, using the current cfg
// values (already merged with env vars and hardcoded defaults) as flag defaults.
// No .Envar() calls are used; environment variables are handled exclusively
// by ParseEnv. Flags therefore win only when explicitly set on the CLI.
//
// The returned func must be called after kingpin.Parse() to apply CLI []string
// slice overrides that cannot be handled with a simple Default binding.
func BindFlags(cfg *Config, kpApp *kingpin.Application, cmd *kingpin.CmdClause) func() {
	bindAppFlags(cfg, cmd)
	bindKubeFlags(cfg, cmd)
	bindLogFlags(cfg, cmd)
	applyAdmission := bindAdmissionWebhookFlags(cfg, cmd)
	applyConversion := bindConversionWebhookFlags(cfg, cmd)
	bindDebugFlags(cfg, kpApp, cmd)

	return func() {
		applyAdmission()
		applyConversion()
	}
}

func bindAppFlags(cfg *Config, cmd *kingpin.CmdClause) {
	cmd.Flag("hooks-dir", "A path to a hooks file structure. Can be set with $SHELL_OPERATOR_HOOKS_DIR.").
		Default(cfg.App.HooksDir).
		StringVar(&cfg.App.HooksDir)

	cmd.Flag("tmp-dir", "A path to store temporary files with data for hooks. Can be set with $SHELL_OPERATOR_TMP_DIR.").
		Default(cfg.App.TempDir).
		StringVar(&cfg.App.TempDir)

	cmd.Flag("listen-address", "Address to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_ADDRESS.").
		Default(cfg.App.ListenAddress).
		StringVar(&cfg.App.ListenAddress)

	cmd.Flag("listen-port", "Port to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_PORT.").
		Default(cfg.App.ListenPort).
		StringVar(&cfg.App.ListenPort)

	cmd.Flag("prometheus-metrics-prefix", "Prefix for Prometheus metrics. Can be set with $SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX.").
		Default(cfg.App.PrometheusMetricsPrefix).
		StringVar(&cfg.App.PrometheusMetricsPrefix)

	cmd.Flag("namespace", "A namespace of a shell-operator. Used to set up validating webhooks. Can be set with $SHELL_OPERATOR_NAMESPACE.").
		Default(cfg.App.Namespace).
		StringVar(&cfg.App.Namespace)
}

func bindKubeFlags(cfg *Config, cmd *kingpin.CmdClause) {
	cmd.Flag("kube-context", "The name of the kubeconfig context to use. Can be set with $KUBE_CONTEXT.").
		Default(cfg.Kube.Context).
		StringVar(&cfg.Kube.Context)

	cmd.Flag("kube-config", "Path to the kubeconfig file. Can be set with $KUBE_CONFIG.").
		Default(cfg.Kube.Config).
		StringVar(&cfg.Kube.Config)

	cmd.Flag("kube-server", "The address and port of the Kubernetes API server. Can be set with $KUBE_SERVER.").
		Default(cfg.Kube.Server).
		StringVar(&cfg.Kube.Server)

	cmd.Flag("kube-client-qps", "QPS for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_QPS.").
		Default(fmt.Sprintf("%g", cfg.Kube.ClientQPS)).
		Float32Var(&cfg.Kube.ClientQPS)

	cmd.Flag("kube-client-burst", "Burst for a rate limiter of a Kubernetes client for hook events. Can be set with $KUBE_CLIENT_BURST.").
		Default(fmt.Sprintf("%d", cfg.Kube.ClientBurst)).
		IntVar(&cfg.Kube.ClientBurst)

	cmd.Flag("object-patcher-kube-client-qps", "QPS for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_QPS.").
		Default(fmt.Sprintf("%g", cfg.ObjectPatcher.KubeClientQPS)).
		Float32Var(&cfg.ObjectPatcher.KubeClientQPS)

	cmd.Flag("object-patcher-kube-client-burst", "Burst for a rate limiter of a Kubernetes client for Object patcher. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_BURST.").
		Default(fmt.Sprintf("%d", cfg.ObjectPatcher.KubeClientBurst)).
		IntVar(&cfg.ObjectPatcher.KubeClientBurst)

	cmd.Flag("object-patcher-kube-client-timeout", "Timeout for object patcher requests to the Kubernetes API server. Can be set with $OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT.").
		Default(cfg.ObjectPatcher.KubeClientTimeout.String()).
		DurationVar(&cfg.ObjectPatcher.KubeClientTimeout)
}

// bindAdmissionWebhookFlags registers validating-webhook flags and returns a fixup
// function that applies the CLI []string ClientCA override after flag parsing.
func bindAdmissionWebhookFlags(cfg *Config, cmd *kingpin.CmdClause) func() {
	cmd.Flag("validating-webhook-configuration-name", "A name of a ValidatingWebhookConfiguration resource. Can be set with $VALIDATING_WEBHOOK_CONFIGURATION_NAME.").
		Default(cfg.Admission.ConfigurationName).
		StringVar(&cfg.Admission.ConfigurationName)

	cmd.Flag("validating-webhook-service-name", "A name of a service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.").
		Default(cfg.Admission.ServiceName).
		StringVar(&cfg.Admission.ServiceName)

	cmd.Flag("validating-webhook-server-cert", "A path to a server certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.").
		Default(cfg.Admission.ServerCert).
		StringVar(&cfg.Admission.ServerCert)

	cmd.Flag("validating-webhook-server-key", "A path to a server private key for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.").
		Default(cfg.Admission.ServerKey).
		StringVar(&cfg.Admission.ServerKey)

	cmd.Flag("validating-webhook-ca", "A path to a CA certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CA.").
		Default(cfg.Admission.CA).
		StringVar(&cfg.Admission.CA)

	// ClientCA is a []string; kingpin's StringsVar always appends so we bind a
	// separate variable and restore the env value when no CLI flag is given.
	envClientCA := cfg.Admission.ClientCA
	var cliClientCA []string
	cmd.Flag("validating-webhook-client-ca", "Paths to client CA certificates for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.").
		StringsVar(&cliClientCA)

	cmd.Flag("validating-webhook-failure-policy", "Default FailurePolicy for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_FAILURE_POLICY.").
		Default(cfg.Admission.FailurePolicy).
		EnumVar(&cfg.Admission.FailurePolicy, "Fail", "Ignore")

	cmd.Flag("validating-webhook-listen-port", "Listen port for ValidatingWebhookConfiguration server. Can be set with $VALIDATING_WEBHOOK_LISTEN_PORT.").
		Default(cfg.Admission.ListenPort).
		StringVar(&cfg.Admission.ListenPort)

	cmd.Flag("validating-webhook-listen-address", "Listen address for ValidatingWebhookConfiguration server. Can be set with $VALIDATING_WEBHOOK_LISTEN_ADDRESS.").
		Default(cfg.Admission.ListenAddress).
		StringVar(&cfg.Admission.ListenAddress)

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
func bindConversionWebhookFlags(cfg *Config, cmd *kingpin.CmdClause) func() {
	cmd.Flag("conversion-webhook-service-name", "A name of a service for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVICE_NAME.").
		Default(cfg.Conversion.ServiceName).
		StringVar(&cfg.Conversion.ServiceName)

	cmd.Flag("conversion-webhook-server-cert", "A path to a server certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_CERT.").
		Default(cfg.Conversion.ServerCert).
		StringVar(&cfg.Conversion.ServerCert)

	cmd.Flag("conversion-webhook-server-key", "A path to a server private key for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_KEY.").
		Default(cfg.Conversion.ServerKey).
		StringVar(&cfg.Conversion.ServerKey)

	cmd.Flag("conversion-webhook-ca", "A path to a CA certificate for conversion webhook. Can be set with $CONVERSION_WEBHOOK_CA.").
		Default(cfg.Conversion.CA).
		StringVar(&cfg.Conversion.CA)

	envClientCA := cfg.Conversion.ClientCA
	var cliClientCA []string
	cmd.Flag("conversion-webhook-client-ca", "Paths to client CA certificates for conversion webhook. Can be set with $CONVERSION_WEBHOOK_CLIENT_CA.").
		StringsVar(&cliClientCA)

	cmd.Flag("conversion-webhook-listen-port", "Listen port for conversion webhook server. Can be set with $CONVERSION_WEBHOOK_LISTEN_PORT.").
		Default(cfg.Conversion.ListenPort).
		StringVar(&cfg.Conversion.ListenPort)

	cmd.Flag("conversion-webhook-listen-address", "Listen address for conversion webhook server. Can be set with $CONVERSION_WEBHOOK_LISTEN_ADDRESS.").
		Default(cfg.Conversion.ListenAddress).
		StringVar(&cfg.Conversion.ListenAddress)

	return func() {
		if len(cliClientCA) > 0 {
			cfg.Conversion.ClientCA = cliClientCA
		} else {
			cfg.Conversion.ClientCA = envClientCA
		}
	}
}

func bindLogFlags(cfg *Config, cmd *kingpin.CmdClause) {
	cmd.Flag("log-level", "Logging level: debug, info, error. Default is info. Can be set with $LOG_LEVEL.").
		Default(cfg.Log.Level).
		StringVar(&cfg.Log.Level)

	cmd.Flag("log-type", "Logging formatter type: json, text or color. Default is text. Can be set with $LOG_TYPE.").
		Default(cfg.Log.Type).
		StringVar(&cfg.Log.Type)

	cmd.Flag("log-no-time", "Disable timestamp logging. Can be set with $LOG_NO_TIME.").
		BoolVar(&cfg.Log.NoTime)

	cmd.Flag("log-proxy-hook-json", "Proxy hook stdout/stderr JSON logging. Can be set with $LOG_PROXY_HOOK_JSON.").
		BoolVar(&cfg.Log.ProxyHookJSON)
}

func bindDebugFlags(cfg *Config, kpApp *kingpin.Application, cmd *kingpin.CmdClause) {
	// Sync the legacy global so debug sub-commands (queue, hook, etc.) get the
	// env/default value before their own --debug-unix-socket flag is parsed.
	DebugUnixSocket = cfg.Debug.UnixSocket
	// The start command binds directly to cfg, not the global.
	cmd.Flag("debug-unix-socket", "A path to a unix socket for a debug endpoint. Can be set with $DEBUG_UNIX_SOCKET.").
		Hidden().
		Default(cfg.Debug.UnixSocket).
		StringVar(&cfg.Debug.UnixSocket)

	cmd.Flag("debug-http-addr", "HTTP address for a debug endpoint. Can be set with $DEBUG_HTTP_SERVER_ADDR.").
		Hidden().
		Default(cfg.Debug.HTTPServerAddr).
		StringVar(&cfg.Debug.HTTPServerAddr)

	cmd.Flag("debug-keep-tmp-files", "Set to true to disable cleanup of temporary files. Can be set with $DEBUG_KEEP_TMP_FILES.").
		Hidden().
		Default("false").
		BoolVar(&cfg.Debug.KeepTempFiles)

	cmd.Flag("debug-kubernetes-api", "Enable client-go debug messages. Can be set with $DEBUG_KUBERNETES_API.").
		Hidden().
		Default("false").
		BoolVar(&cfg.Debug.KubernetesAPI)

	// A command to show help about hidden debug-* flags.
	kpApp.Command("debug-options", "Show help for debug flags of a start command.").Hidden().PreAction(func(_ *kingpin.ParseContext) error {
		context, err := kpApp.ParseContext([]string{"start"})
		if err != nil {
			return err
		}

		usageTemplate := `{{define "FormatCommand"}}\
{{if .FlagSummary}} {{.FlagSummary}}{{end}}\
{{range .Args}} {{if not .Required}}[{{end}}<{{.Name}}>{{if .Value|IsCumulative}}...{{end}}{{if not .Required}}]{{end}}{{end}}\
{{end}}\

{{define "FormatUsage"}}\
{{template "FormatCommand" .}}{{if .Commands}} <command> [<args> ...]{{end}}
{{if .Help}}
{{.Help|Wrap 0}}\
{{end}}\
{{end}}\

usage: {{.App.Name}}{{template "FormatUsage" .App}}
Debug flags:
{{range .Context.Flags}}\
{{if and (ge .Name "debug-") (le .Name "debug-zzz") }}\
{{if .Short}}-{{.Short|Char}}, {{end}}--{{.Name}}{{if not .IsBoolFlag}}={{.FormatPlaceHolder}}{{end}}
        {{.Help}}
{{end}}\
{{end}}\
`

		if err := kpApp.UsageForContextWithTemplate(context, 2, usageTemplate); err != nil {
			panic(err)
		}

		os.Exit(0)
		return nil
	})
}
