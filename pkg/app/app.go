package app

import (
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var AppName = "shell-operator"
var AppDescription = "Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule."

var Version = "dev"

var DebugMessages = "no"

var WorkingDir = ""
var TempDir = "/tmp/shell-operator"
var KubeContext = ""
var KubeConfig = ""
var ListenAddress, _ = net.ResolveTCPAddr("tcp", "0.0.0.0:9115")
var JqLibraryPath = ""

// Use info level with timestamps and a text output by default
var LogLevel = "info"
var LogNoTime = false
var LogType = "text"

// SetupGlobalSettings init global flags with default values
func SetupGlobalSettings(kpApp *kingpin.Application) {
	kpApp.Flag("debug", "set to yes to turn on debug messages").
		Envar("SHELL_OPERATOR_DEBUG").
		Default(DebugMessages).
		StringVar(&DebugMessages)

	kpApp.Flag("working-dir", "a path to a hooks file structure").
		Envar("SHELL_OPERATOR_WORKING_DIR").
		Default(WorkingDir).
		StringVar(&WorkingDir)

	kpApp.Flag("tmp-dir", "a path to store temporary files with data for hooks").
		Envar("SHELL_OPERATOR_TMP_DIR").
		Default(TempDir).
		StringVar(&TempDir)

	kpApp.Flag("kube-context", "The name of the kubeconfig context to use (can be set with $SHELL_OPERATOR_KUBE_CONTEXT).").
		Envar("SHELL_OPERATOR_KUBE_CONTEXT").
		Default(KubeContext).
		StringVar(&KubeContext)

	kpApp.Flag("kube-config", "Path to the kubeconfig file (can be set with $SHELL_OPERATOR_KUBE_CONFIG).").
		Envar("SHELL_OPERATOR_KUBE_CONFIG").
		Default(KubeConfig).
		StringVar(&KubeConfig)

	kpApp.Flag("listen-address", "Address and port to use for HTTP serving.").
		Envar("SHELL_OPERATOR_LISTEN_ADDRESS").
		Default(ListenAddress.String()).
		TCPVar(&ListenAddress)

	kpApp.Flag("jq-library-path", "Prepend directory to the search list for jq modules (-L flag). (Can be set with $JQ_LIBRARY_PATH).").
		Envar("JQ_LIBRARY_PATH").
		Default(JqLibraryPath).
		StringVar(&JqLibraryPath)

	kpApp.Flag("log-level", "Logging level: debug, info, error. Default is info.").
		Envar("LOG_LEVEL").
		Default(LogLevel).
		StringVar(&LogLevel)
	kpApp.Flag("log-type", "Logging formatter type: json, text or color. Default is text.").
		Envar("LOG_TYPE").
		Default(LogType).
		StringVar(&LogType)
	kpApp.Flag("log-no-time", "Disable timestamp logging if flag is present. Useful when output is redirected to logging system that already adds timestamps.").
		Envar("LOG_NO_TIME").
		BoolVar(&LogNoTime)
}

// SetupLogging sets logging output
func SetupLogging() {
	switch LogType {
	case "json":
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, DisableColors: true})
	case "color":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, ForceColors: true})
	default:
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	}

	switch strings.ToLower(LogLevel) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
