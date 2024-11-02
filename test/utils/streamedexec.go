//go:build test
// +build test

// TODO: remove useless code

package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

type CommandOptions struct {
	OutputLineHandler func(string)
	StopCh            chan struct{}
}

type CommandError struct {
	CommandError error
	ExitCode     int
}

func (c *CommandError) Error() string {
	return c.CommandError.Error()
}

func StreamedExecCommand(cmd *exec.Cmd, opts CommandOptions) error {
	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stdout: %s", err)
	}

	stderrReadPipe, stderrWritePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create os pipe for stderr: %s", err)
	}

	// outputReader := io.MultiReader(stdoutReadPipe, stderrReadPipe)

	fmt.Printf("gexec.Start\n")
	session, err := gexec.Start(cmd, stdoutWritePipe, stderrWritePipe)
	if err != nil {
		return fmt.Errorf("error starting command: %s", err)
	}

	var stopMsg string
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if opts.StopCh != nil {
			select {
			case <-opts.StopCh:
				stopMsg = "command is stopped"
				session.Interrupt().Wait(time.Second * 30)
			case <-session.Exited:
			}
		} else {
			<-session.Exited
		}

		// Initiate EOF to stop consumeLines
		stdoutWritePipe.Close()
		stderrWritePipe.Close()
	}()

	go func() {
		defer wg.Done()
		defer gexecRecover(session)
		defer ginkgo.GinkgoRecover()
		consumeLines(stdoutReadPipe, session, opts)
	}()

	go func() {
		defer wg.Done()
		defer gexecRecover(session)
		defer ginkgo.GinkgoRecover()
		consumeLines(stderrReadPipe, session, opts)
	}()

	wg.Wait()

	if exitCode := session.ExitCode(); exitCode != 0 {
		cmdErr := fmt.Errorf("command failed, exit code %d", exitCode)
		if stopMsg != "" {
			cmdErr = fmt.Errorf(stopMsg)
		}
		return &CommandError{
			CommandError: cmdErr,
			ExitCode:     exitCode,
		}
	}
	return nil
}

func consumeLines(r io.Reader, _ *gexec.Session, opts CommandOptions) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		opts.OutputLineHandler(scanner.Text())
	}
}

func gexecRecover(session *gexec.Session) {
	e := recover()
	if e != nil && session != nil {
		session.Kill()
	}
}
