package executor

// Some information about docker and pid1 process and zombie problem:
// https://blog.phusion.nl/2015/01/20/docker-and-the-pid-1-zombie-reaping-problem/
// The code hereafter is from go-reaper (https://github.com/ramr/go-reaper) with small change:
// - use rlog instead of fmt
// - lock reaper when cmd.Run or cmd.Output are called

/*  Note:  This is a *nix only implementation.  */

//  Prefer #include style directives.
import (
	"os"
	"os/signal"
	"syscall"

	"github.com/romana/rlog"
)

type Config struct {
	Pid              int
	Options          int
	DisablePid1Check bool
}

//  Handle death of child (SIGCHLD) messages. Pushes the signal onto the
//  notifications channel if there is a waiter.
func sigChildHandler(notifications chan os.Signal) {
	rlog.Debugf("Start SIGCHLD handler")
	var sigs = make(chan os.Signal, 3)
	signal.Notify(sigs, syscall.SIGCHLD)

	for {
		var sig = <-sigs
		select {
		case notifications <- sig: /*  published it.  */
		default:
			/*
			 *  Notifications channel full - drop it to the
			 *  floor. This ensures we don't fill up the SIGCHLD
			 *  queue. The reaper just waits for any child
			 *  process (pid=-1), so we ain't loosing it!! ;^)
			 */
		}
	}

} /*  End of function  sigChildHandler.  */

//  Be a good parent - clean up behind the children.
func reapChildren(config Config) {
	rlog.Debugf("Start WAIT4 loop")

	var notifications = make(chan os.Signal, 1)

	go sigChildHandler(notifications)

	pid := config.Pid
	opts := config.Options

	for {
		<-notifications
		func() {
			// Acquire lock
			ExecutorLock.Lock()
			defer ExecutorLock.Unlock()

			for {
				var wstatus syscall.WaitStatus

				/*
				 *  Reap 'em, so that zombies don't accumulate.
				 *  Plants vs. Zombies!!
				 */
				pid, err := syscall.Wait4(pid, &wstatus, opts, nil)
				for syscall.EINTR == err {
					pid, err = syscall.Wait4(pid, &wstatus, opts, nil)
				}

				if syscall.ECHILD == err {
					break
				}

				rlog.Debugf(" - Grim reaper cleanup: pid=%d, wstatus=%+v\n",
					pid, wstatus)

			}
		}()
	}

} /*   End of function  reapChildren.  */

/*
 *  ======================================================================
 *  Section: Exported functions
 *  ======================================================================
 */

//  Normal entry point for the reaper code. Start reaping children in the
//  background inside a goroutine.
func Reap() {
	/*
	 *  Only reap processes if we are taking over init's duties aka
	 *  we are running as pid 1 inside a docker container. The default
	 *  is to reap all processes.
	 */
	Start(Config{
		Pid:              -1,
		Options:          0,
		DisablePid1Check: false,
	})

} /*  End of [exported] function  Reap.  */

//  Entry point for invoking the reaper code with a specific configuration.
//  The config allows you to bypass the pid 1 checks, so handle with care.
//  The child processes are reaped in the background inside a goroutine.
func Start(config Config) {
	/*
	 *  Start the Reaper with configuration options. This allows you to
	 *  reap processes even if the current pid isn't running as pid 1.
	 *  So ... use with caution!!
	 *
	 *  In most cases, you are better off just using Reap() as that
	 *  checks if we are running as Pid 1.
	 */
	if !config.DisablePid1Check {
		mypid := os.Getpid()
		if 1 != mypid {
			rlog.Debugf(" - Grim reaper disabled, pid not 1\n")
			return
		}
	}

	/*
	 *  Ok, so either pid 1 checks are disabled or we are the grandma
	 *  of 'em all, either way we get to play the grim reaper.
	 *  You will be missed, Terry Pratchett!! RIP
	 */
	go reapChildren(config)

} /*  End of [exported] function  Start.  */
