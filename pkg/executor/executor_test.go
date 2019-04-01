package executor

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/romana/rlog"
)

// ошибка waitid: no child process приходит от bash, который запускается основным процессом,
// если reaper почистил дочерний процесс этого bash-а.
// Нельзя запускать reaper параллельно с работающими cmd.Run или cmd.Output, потому что
// даже если отловить ECHILD, то не получится узнать exitCode — его уже собрал reaper :(
func TestCmdRun(t *testing.T) {
	//t.SkipNow()
	os.Setenv("RLOG_LOG_LEVEL", "DEBUG")
	rlog.UpdateEnv()

	config := Config{
		Pid:              -1,
		Options:          0,
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
	rlog.Infof("run command:", name, args)
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
		rlog.Debug("start wait")
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
			rlog.Debugf("SyscallError %+v", errv)
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// This will happen (in OSX) if `name` is not available in $PATH,
			// in this situation, exit code could not be get, and stderr will be
			// empty string very likely, so we use the default fail code, and format err
			// to string and set to stderr
			rlog.Debugf("Could not get exit code for failed program: %v, %+v", name, args)
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
	rlog.Debugf("command result, stdout: %v, stderr: %v, exitCode: %v, echild: %v", stdout, stderr, exitCode, echild)
	return
}

// Проверка блокировок между reaper и вызовом cmd.Run
// Запуск 10 горутин с вызовом bash и рипера
func TestExecutorCmdRun(t *testing.T) {
	t.SkipNow()
	os.Setenv("RLOG_LOG_LEVEL", "DEBUG")
	rlog.UpdateEnv()

	config := Config{
		Pid:              -1,
		Options:          0,
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
	rlog.Infof("run go routines for: %v %#v", name, args)

	stopCh := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			var outbuf, errbuf bytes.Buffer
			cmd := exec.Command(name, args...)
			cmd.Stdout = &outbuf
			cmd.Stderr = &errbuf

			err := Run(cmd, true)

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
					rlog.Debugf("Could not get exit code for failed program: %v, %+v", name, args)
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
			rlog.Debugf("command result, stdout: %v, stderr: %v, exitCode: %v, echild: %v", stdout, stderr, exitCode)
			stopCh <- true
		}()

	}

	counter := 0
WAIT_LOOP:
	for {
		select {
		case <-stopCh:
			rlog.Info("Got stopCh")
			counter++
			if counter == 10 {
				break WAIT_LOOP
			}

		}
	}

	return
}
