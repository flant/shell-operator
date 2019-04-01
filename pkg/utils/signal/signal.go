package utils

import (
	"github.com/romana/rlog"
	"os"
	"os/signal"
	"syscall"
)

func WaitForProcessInterruption() {
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-interruptCh:
			rlog.Infof("Grace shutdown with %s signal", sig.String())
			return
		}
	}
}
