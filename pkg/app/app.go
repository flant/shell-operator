package app

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"net"
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
}
