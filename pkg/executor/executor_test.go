package executor

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

// Not really a test, more like a proof of concept.
//
// Reaper should not work in parallel with cmd.Run or cmd.Output:
// - cmd.Run do wait children, but reaper will clean them
// - reaper prevent to get real exitCode from cmd.Run
func Test_CmdRun_And_Reaper_PoC_Code(t *testing.T) {
	t.SkipNow()
	log.SetLevel(log.DebugLevel)

	config := Config{
		Pid:              -1,
		DisablePid1Check: true,
	}
	go Start(config)

	var exitCode int
	defaultFailedCode := 1

	name := "/bin/bash"
	//name := "/bin/bashk" // err будет PathError
	args := []string{
		"-c",
		"ls -la; /bin/bash -c 'sleep 0.2; exit 1';  exit 2",
		// "ls -la; sleep 1; exit 2", // no waitid error
	}
	log.Infof("run command: %s %+v", name, args)
	var outbuf, errbuf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	//err := cmd.Run()

	err := func(c *exec.Cmd) error {
		// с блоком всё работает, получаем exitCode 2
		// executor.ExecutorLock.Lock()
		// defer executor.ExecutorLock.Unlock()
		if err := c.Start(); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		log.Debug("start wait")
		return c.Wait()
	}(cmd)

	stdout := outbuf.String()
	stderr := errbuf.String()

	echild := false

	if err != nil {
		// try to get the exit code
		if errv, ok := err.(*os.SyscallError); ok {
			if sysErr, ok := errv.Err.(syscall.Errno); ok {
				if sysErr == syscall.ECHILD {
					echild = true
				}
			}
			log.Debugf("SyscallError %+v", errv)
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			log.Debugf("Could not get exit code for failed program: %v, %+v", name, args)
			exitCode = defaultFailedCode
			if stderr == "" {
				stderr = err.Error()
			}
		}
	} else {
		// success, exitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}
	log.Debugf("command result, stdout: %v, stderr: %v, exitCode: %v, echild: %v", stdout, stderr, exitCode, echild)
	return
}

// Check that reaper and cmd.Run are locked.
// Run 10 go-routines that execute bash.
func Test_Executor_Reaper_And_CmdRun_AreLocked(t *testing.T) {
	t.SkipNow()
	log.SetLevel(log.DebugLevel)

	config := Config{
		Pid:              -1,
		DisablePid1Check: true,
	}
	go Start(config)

	var exitCode int
	defaultFailedCode := 1

	name := "/bin/bash"
	args := []string{
		"-c",
		"ls -la; /bin/bash -c 'sleep 0.2; exit 1';  exit 2",
	}
	log.Infof("run go routines for: %v %#v", name, args)

	stopCh := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			var outbuf, errbuf bytes.Buffer
			cmd := exec.Command(name, args...)
			cmd.Stdout = &outbuf
			cmd.Stderr = &errbuf

			err := Run(cmd)

			stdout := outbuf.String()
			stderr := errbuf.String()

			if err != nil {
				// try to get the exit code
				if exitError, ok := err.(*exec.ExitError); ok {
					ws := exitError.Sys().(syscall.WaitStatus)
					exitCode = ws.ExitStatus()
				} else {
					// This will happen (in OSX) if `name` is not available in $PATH,
					// in this situation, exit code could not be get, and stderr will be
					// empty string very likely, so we use the default fail code, and format err
					// to string and set to stderr
					log.Debugf("Could not get exit code for failed program: %v, %+v", name, args)
					exitCode = defaultFailedCode
					if stderr == "" {
						stderr = err.Error()
					}
				}
			} else {
				// success, exitCode should be 0 if go is ok
				ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			}
			log.Debugf("command result, stdout: %v, stderr: %v, exitCode: %v", stdout, stderr, exitCode)
			stopCh <- true
		}()

	}

	counter := 0
WAIT_LOOP:
	for {
		select {
		case <-stopCh:
			log.Info("Got stopCh")
			counter++
			if counter == 10 {
				break WAIT_LOOP
			}

		}
	}

	return
}
