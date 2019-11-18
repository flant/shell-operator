// +build test

package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

func main() {
	app.SetupLogging()
	log.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		os.Exit(1)
	}

	if os.Args[1] == "nocb" {
		fmt.Printf("Start nocb variant\n")
		utils_signal.WaitForProcessInterruption()
	}

	if os.Args[1] == "withcb" {
		fmt.Printf("Start withcb variant\n")
		// Block action by waiting signals from OS.
		utils_signal.WaitForProcessInterruption(func() {
			os.Exit(0)
		})
	}

	if os.Args[1] == "withlongcb" {
		fmt.Printf("Start withlongcb variant\n")
		// Block action by waiting signals from OS.
		utils_signal.WaitForProcessInterruption(func() {
			// we should do something very quickly to not lock a signal handler
		})
	}

	fmt.Fprintf(os.Stderr, "bad argument: %s\n", os.Args[1])
	os.Exit(2)
}
