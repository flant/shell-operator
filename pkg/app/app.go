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

var ListenAddress = "0.0.0.0"
var ListenPort = "9115"
var HookMetricsListenPort = ""

var PrometheusMetricsPrefix = "shell_operator_"

// DefineStartCommandFlags set shell-operator flags for cmd
func DefineStartCommandFlags(kpApp *kingpin.Application, cmd *kingpin.CmdClause) {
	cmd.Flag("hooks-dir", "A path to a hooks file structure. Can be set with $SHELL_OPERATOR_HOOKS_DIR.").
		Envar("SHELL_OPERATOR_HOOKS_DIR").
		Default(HooksDir).
		StringVar(&HooksDir)

	cmd.Flag("tmp-dir", "A path to store temporary files with data for hooks. Can be set with $SHELL_OPERATOR_TMP_DIR.").
		Envar("SHELL_OPERATOR_TMP_DIR").
		Default(TempDir).
		StringVar(&TempDir)

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

	cmd.Flag("hook-metrics-listen-port", "Port to use to serve hooks’ custom metrics to Prometheus. Can be set with $SHELL_OPERATOR_HOOK_METRICS_LISTEN_PORT. Equal to listen-port if empty.").
		Envar("SHELL_OPERATOR_HOOK_METRICS_LISTEN_PORT").
		Default(HookMetricsListenPort).
		StringVar(&HookMetricsListenPort)

	DefineKubeClientFlags(cmd)
	DefineJqFlags(cmd)
	DefineLoggingFlags(cmd)
	DefineDebugFlags(kpApp, cmd)
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
