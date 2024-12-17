package utils

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// WaitForProcessInterruption wait for SIGINT or SIGTERM and run a callback function.
//
// First signal start a callback function, which should call os.Exit(0).
// Next signal will call os.Exit(128 + signal-value).
// If no cb is given,
func WaitForProcessInterruption(cb ...func()) {
	allowedCount := 1
	interruptCh := make(chan os.Signal, 1)

	forcedExit := func(s os.Signal) {
		log.Info("Forced shutdown by signal", slog.String("name", s.String()))

		signum := 0
		if v, ok := s.(syscall.Signal); ok {
			signum = int(v)
		}
		os.Exit(128 + signum)
	}

	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)
	for {
		sig := <-interruptCh
		allowedCount--
		switch allowedCount {
		case 0:
			if len(cb) > 0 {
				log.Info("Grace shutdown by signal", slog.String("name", sig.String()))
				cb[0]()
			} else {
				forcedExit(sig)
			}
		case -1:
			forcedExit(sig)
		}
	}
}
