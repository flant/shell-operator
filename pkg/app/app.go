package app

import "gopkg.in/alecthomas/kingpin.v2"

var AppName = "shell-operator"
var AppDescription = "Run your custom cluster-wide scripts in reaction to Kubernetes events or on schedule."

var Version = "dev"

var DebugMessages = "no"

// SetupGlobalSettings init global flags with default values
func SetupGlobalSettings(kpApp *kingpin.Application) {
	kpApp.Flag("debug", "set to yes to turn on debug messages").
		Envar("SHELL_OPERATOR_DEBUG").
		Default(DebugMessages).
		StringVar(&DebugMessages)

}