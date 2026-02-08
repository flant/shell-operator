package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/kube-client/klogtolog"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/jq"
)

func main() {
	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, app.Version, app.AppDescription))

	logger := log.NewLogger()
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
		fmt.Printf("%s %s\n%s\n", app.AppName, app.Version, jq.Info())
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start shell-operator.").
		Default().
		Action(start(logger))
	app.DefineStartCommandFlags(kpApp, startCmd)

	debug.DefineDebugCommands(kpApp)
	debug.DefineDebugCommandsSelf(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
