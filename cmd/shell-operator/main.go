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
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

func main() {
	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, app.Version, app.AppDescription))

	logger := log.NewLogger(log.Options{})
	log.SetDefault(logger)

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(app.OperatorUsageTemplate(app.AppName))

	// Initialize klog wrapper when all values are parsed
	kpApp.Action(func(_ *kingpin.ParseContext) error {
		klogtolog.InitAdapter(app.DebugKubernetesAPI, logger.Named("klog"))
		return nil
	})

	// print version
	kpApp.Command("version", "Show version.").Action(func(_ *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.Version)
		fl := jq.NewFilter()
		fmt.Println(fl.FilterInfo())
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start shell-operator.").
		Default().
		Action(func(_ *kingpin.ParseContext) error {
			app.AppStartMessage = fmt.Sprintf("%s %s", app.AppName, app.Version)

			// Init logging and initialize a ShellOperator instance.
			operator, err := shell_operator.Init(logger.Named("shell-operator"))
			if err != nil {
				os.Exit(1)
			}
			operator.Start()

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				operator.Shutdown()
				os.Exit(1)
			})

			return nil
		})
	app.DefineStartCommandFlags(kpApp, startCmd)

	debug.DefineDebugCommands(kpApp)
	debug.DefineDebugCommandsSelf(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
