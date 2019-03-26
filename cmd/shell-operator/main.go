package main

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
	operator "github.com/flant/shell-operator/pkg/shell-operator"
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
	kpApp.
		Command("start", "Start events processing.").
		Action(func(c *kingpin.ParseContext) error {
			err := operator.Start()
			if err != nil {
				os.Exit(1)
			}
			return nil
	})

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
