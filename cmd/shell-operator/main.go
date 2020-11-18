package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/flant/shell-operator/pkg/debug"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
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
			// Init logging subsystem.
			app.SetupLogging()
			log.Infof("%s %s", app.AppName, app.Version)
			// Init rand generator.
			rand.Seed(time.Now().UnixNano())

			defaultOperator := shell_operator.DefaultOperator()
			err := shell_operator.InitAndStart(defaultOperator)
			if err != nil {
				os.Exit(1)
			}

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				defaultOperator.Shutdown()
				os.Exit(1)
			})

			return nil
		})
	app.DefineStartCommandFlags(kpApp, startCmd)

	debug.DefineDebugCommands(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
