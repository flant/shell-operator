// +build test

package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

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

	//outputReader := io.MultiReader(stdoutReadPipe, stderrReadPipe)

	fmt.Printf("gexec.Start\n")
	session, err := gexec.Start(cmd, stdoutWritePipe, stderrWritePipe)
	if err != nil {
		return fmt.Errorf("error starting command: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if opts.StopCh != nil {
			select {
			case <-opts.StopCh:
				session.Kill()
			case <-session.Exited:
			}
		} else {
			<-session.Exited
		}

		fmt.Printf("Session exited\n")

		// Initiate EOF for consumeLines
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
		return &CommandError{
			CommandError: fmt.Errorf("command failed, exit code %d", exitCode),
			ExitCode:     exitCode,
		}
	}
	return nil
}

func consumeLines(r io.Reader, session *gexec.Session, opts CommandOptions) {
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
