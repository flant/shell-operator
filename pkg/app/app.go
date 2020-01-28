package app

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
)

var AppName = "shell-operator"
var AppDescription = "Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule."

var Version = "dev"

var HooksDir = ""
var TempDir = "/tmp/shell-operator"
var KubeContext = ""
var KubeConfig = ""
var ListenAddress = "0.0.0.0"
var ListenPort = "9115"

var PrometheusMetricsPrefix = "shell_operator_"

// DefineAppFlags set shell-operator flags for cmd
func DefineAppFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("hooks-dir", "A path to a hooks file structure. Can be set with $SHELL_OPERATOR_HOOKS_DIR.").
		Envar("SHELL_OPERATOR_HOOKS_DIR").
		Default(HooksDir).
		StringVar(&HooksDir)

	cmd.Flag("tmp-dir", "A path to store temporary files with data for hooks. Can be set with $SHELL_OPERATOR_HOOKS_DIR.").
		Envar("SHELL_OPERATOR_TMP_DIR").
		Default(TempDir).
		StringVar(&TempDir)

	cmd.Flag("kube-context", "The name of the kubeconfig context to use. Can be set with $SHELL_OPERATOR_KUBE_CONTEXT.").
		Envar("SHELL_OPERATOR_KUBE_CONTEXT").
		Default(KubeContext).
		StringVar(&KubeContext)

	cmd.Flag("kube-config", "Path to the kubeconfig file. Can be set with $SHELL_OPERATOR_KUBE_CONFIG.").
		Envar("SHELL_OPERATOR_KUBE_CONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)

	cmd.Flag("listen-address", "Address to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_ADDRESS.").
		Envar("SHELL_OPERATOR_LISTEN_ADDRESS").
		Default(ListenAddress).
		StringVar(&ListenAddress)
	cmd.Flag("listen-port", "Port to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_PORT.").
		Envar("SHELL_OPERATOR_LISTEN_PORT").
		Default(ListenPort).
		StringVar(&ListenPort)

	cmd.Flag("prometheus-metrics-prefix", "Prefix for Prometheus metrics. Can be set with $SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX.").
		Envar("SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX").
		Default(PrometheusMetricsPrefix).
		StringVar(&PrometheusMetricsPrefix)
}

func OperatorUsageTemplate(appName string) string {
	return kingpin.DefaultUsageTemplate + fmt.Sprintf(`

Use "%s debug-options" for a list of debug options for start command.
`, appName)
}

// CommandWithDefaultUsageTemplate is used to workaround an absence of per-command usage templates
func CommandWithDefaultUsageTemplate(kpApp *kingpin.Application, name, help string) *kingpin.CmdClause {
	return kpApp.Command(name, help).PreAction(func(context *kingpin.ParseContext) error {
		kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
		return nil
	})
}
