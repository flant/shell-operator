package main

import (
"fmt"
"os"

"github.com/deckhouse/deckhouse/pkg/log"
"gopkg.in/alecthomas/kingpin.v2"

"github.com/flant/kube-client/klogtolog"
"github.com/flant/shell-operator/pkg/app"
"github.com/flant/shell-operator/pkg/debug"
"github.com/flant/shell-operator/pkg/filter/jq"
)

func main() {
	// Build the config from defaults and environment variables.
	// Flag defaults will be set to these pre-merged values so that an explicit
	// CLI flag wins, an env var wins over the hardcoded default, but no flag
	// ever reads the environment directly (.Envar() is not used).
	cfg := app.NewConfig()
	if err := app.ParseEnv(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, app.Version, app.AppDescription))

	logger := log.NewLogger()
	log.SetDefault(logger)

	kpApp.UsageTemplate(app.OperatorUsageTemplate(app.AppName))

	// klog adapter uses the final (post-parse) value of cfg.Debug.KubernetesAPI.
	kpApp.Action(func(_ *kingpin.ParseContext) error {
klogtolog.InitAdapter(cfg.Debug.KubernetesAPI, logger.Named("klog"))
return nil
})

	kpApp.Command("version", "Show version.").Action(func(_ *kingpin.ParseContext) error {
fmt.Printf("%s %s\n", app.AppName, app.Version)
fl := jq.NewFilter()
		fmt.Println(fl.FilterInfo())
		return nil
	})

	startCmd := kpApp.Command("start", "Start shell-operator.").
		Default().
		Action(start(logger, cfg))

	// Register all CLI flags; each flag's .Default() reflects the env-merged value.
// No .Envar() - environment is handled exclusively by ParseEnv above.
applySliceFlags := app.BindFlags(cfg, kpApp, startCmd)

// Apply []string slice flag overrides before the start action runs.
// PreAction executes after flag parsing but before Action.
startCmd.PreAction(func(_ *kingpin.ParseContext) error {
applySliceFlags()
return nil
})

debug.DefineDebugCommands(kpApp)
debug.DefineDebugCommandsSelf(kpApp)

kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
