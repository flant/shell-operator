package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	operator "github.com/flant/shell-operator/pkg/shell-operator"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

func main() {
	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, app.Version, app.AppDescription))

	// global defaults
	app.SetupGlobalSettings(kpApp)

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("%s %s\n", app.AppName, app.Version)
		return nil
	})

	// start main loop
	kpApp.Command("start", "Start events processing.").
		Action(func(c *kingpin.ParseContext) error {
			app.SetupLogging()
			log.Infof("%s %s", app.AppName, app.Version)

			// Be a good parent - clean up after the child processes
			// in case if shell-operator is a PID1.
			go executor.Reap()

			operator.InitHttpServer(app.ListenAddress)

			err := operator.Init()
			if err != nil {
				os.Exit(1)
			}

			operator.Run()

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption()

			return nil
		})

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
