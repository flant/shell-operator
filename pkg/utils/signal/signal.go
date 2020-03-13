package utils

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
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
		log.Infof("Forced shutdown by '%s' signal", s.String())

		signum := 0
		switch v := s.(type) {
		case syscall.Signal:
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
				log.Infof("Grace shutdown by '%s' signal", sig.String())
				cb[0]()
			} else {
				forcedExit(sig)
			}
		case -1:
			forcedExit(sig)
		}
	}
}
