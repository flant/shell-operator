package app

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	AppName         = "shell-operator"
	AppDescription  = "Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule."
	AppStartMessage = "shell-operator"
)

var Version = "dev"

var (
	HooksDir = "hooks"
	TempDir  = "/tmp/shell-operator"
)

var (
	Namespace             = ""
	ListenAddress         = "0.0.0.0"
	ListenPort            = "9115"
	HookMetricsListenPort = ""
)

var PrometheusMetricsPrefix = "shell_operator_"

type FlagInfo struct {
	Name   string
	Help   string
	Envar  string
	Define bool
}

var CommonFlagsInfo = map[string]FlagInfo{
	"hooks-dir": {
		"hooks-dir",
		"A path to a hooks file structure. Can be set with $SHELL_OPERATOR_HOOKS_DIR.",
		"SHELL_OPERATOR_HOOKS_DIR",
		true,
	},
	"tmp-dir": {
		"tmp-dir",
		"A path to store temporary files with data for hooks. Can be set with $SHELL_OPERATOR_TMP_DIR.",
		"SHELL_OPERATOR_TMP_DIR",
		true,
	},
	"listen-address": {
		"listen-address",
		"Address to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_ADDRESS.",
		"SHELL_OPERATOR_LISTEN_ADDRESS",
		true,
	},
	"listen-port": {
		"listen-port",
		"Port to use to serve metrics to Prometheus. Can be set with $SHELL_OPERATOR_LISTEN_PORT.",
		"SHELL_OPERATOR_LISTEN_PORT",
		true,
	},
	"prometheus-metrics-prefix": {
		"prometheus-metrics-prefix",
		"Prefix for Prometheus metrics. Can be set with $SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX.",
		"SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX",
		true,
	},
	"hook-metrics-listen-port": {
		"hook-metrics-listen-port",
		"Port to use to serve hooks’ custom metrics to Prometheus. Can be set with $SHELL_OPERATOR_HOOK_METRICS_LISTEN_PORT. Equal to listen-port if empty.",
		"SHELL_OPERATOR_HOOK_METRICS_LISTEN_PORT",
		true,
	},
	"namespace": {
		"namespace",
		"A namespace of a shell-operator. Used to setup validating webhooks. Can be set with $SHELL_OPERATOR_NAMESPACE.",
		"SHELL_OPERATOR_NAMESPACE",
		true,
	},
}

// DefineStartCommandFlags set shell-operator flags for cmd
func DefineStartCommandFlags(kpApp *kingpin.Application, cmd *kingpin.CmdClause) {
	var flag FlagInfo

	flag = CommonFlagsInfo["hooks-dir"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(HooksDir).
			StringVar(&HooksDir)
	}

	flag = CommonFlagsInfo["tmp-dir"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(TempDir).
			StringVar(&TempDir)
	}

	flag = CommonFlagsInfo["listen-address"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(ListenAddress).
			StringVar(&ListenAddress)
	}

	flag = CommonFlagsInfo["listen-port"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(ListenPort).
			StringVar(&ListenPort)
	}

	flag = CommonFlagsInfo["prometheus-metrics-prefix"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(PrometheusMetricsPrefix).
			StringVar(&PrometheusMetricsPrefix)
	}

	flag = CommonFlagsInfo["hook-metrics-listen-port"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(HookMetricsListenPort).
			StringVar(&HookMetricsListenPort)
	}

	flag = CommonFlagsInfo["namespace"]
	if flag.Define {
		cmd.Flag(flag.Name, flag.Help).
			Envar(flag.Envar).
			Default(Namespace).
			StringVar(&Namespace)
	}

	DefineKubeClientFlags(cmd)
	DefineValidatingWebhookFlags(cmd)
	DefineConversionWebhookFlags(cmd)
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
