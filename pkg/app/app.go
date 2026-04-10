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

var Version = "v1.2.0-dev"

func OperatorUsageTemplate(appName string) string {
	return kingpin.DefaultUsageTemplate + fmt.Sprintf(`

Use "%s debug-options" for a list of debug options for start command.
`, appName)
}

// CommandWithDefaultUsageTemplate is used to workaround an absence of per-command usage templates
func CommandWithDefaultUsageTemplate(kpApp *kingpin.Application, name, help string) *kingpin.CmdClause {
	return kpApp.Command(name, help).PreAction(func(_ *kingpin.ParseContext) error {
kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
return nil
})
}
