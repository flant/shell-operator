package main

import (
	"fmt"
	"os"

	"github.com/flant/shell-operator/pkg/debug"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

func main() {
	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, app.Version, app.AppDescription))

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(app.OperatorUsageTemplate(app.AppName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.Version)
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start shell-operator.").
		Default().
		Action(func(c *kingpin.ParseContext) error {
			app.SetupLogging()
			log.Infof("%s %s", app.AppName, app.Version)

			// Be a good parent - clean up after the child processes
			// in case if shell-operator is a PID1.
			go executor.Reap()

			defaultOperator := shell_operator.DefaultOperator()
			err := shell_operator.InitAndStart(defaultOperator)
			if err != nil {
				os.Exit(1)
			}

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				defaultOperator.Stop()
			})

			return nil
		})
	app.SetupStartCommandFlags(kpApp, startCmd)

	debug.DefineDebugCommands(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
