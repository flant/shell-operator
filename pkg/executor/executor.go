package executor

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type CmdUsage struct {
	Sys    time.Duration
	User   time.Duration
	MaxRss int64
}

func Run(cmd *exec.Cmd) error {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	return cmd.Run()
}

func RunAndLogLines(cmd *exec.Cmd, logLabels map[string]string) (*CmdUsage, error) {
	// TODO observability
	logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))
	stdoutLogEntry := logEntry.WithField("output", "stdout")
	stderrLogEntry := logEntry.WithField("output", "stderr")

	logEntry.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	var wg sync.WaitGroup

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		if app.LogProxyHookJSON {
			proxyJSONLogs(stdout, stdoutLogEntry)
		} else {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				stdoutLogEntry.Info(scanner.Text())
			}
		}
	}()

	go func() {
		defer wg.Done()
		if app.LogProxyHookJSON {
			proxyJSONLogs(stderr, stderrLogEntry)
		} else {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				stderrLogEntry.Info(scanner.Text())
			}
		}
	}()

	wg.Wait()

	err = cmd.Wait()

	var usage *CmdUsage
	if cmd.ProcessState != nil {
		usage = &CmdUsage{
			Sys:  cmd.ProcessState.SystemTime(),
			User: cmd.ProcessState.UserTime(),
		}
		// FIXME Maxrss is Unix specific.
		sysUsage := cmd.ProcessState.SysUsage()
		if v, ok := sysUsage.(*syscall.Rusage); ok {
			// v.Maxrss is int32 on arm/v7
			usage.MaxRss = v.Maxrss
		}
	}

	return usage, err
}

func proxyJSONLogs(r io.ReadCloser, logEntry *log.Entry) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var line interface{}
		if err := json.Unmarshal([]byte(scanner.Text()), &line); err != nil {
			logEntry.Debugf("unmarshal json log line: %v", err)
			// fall back to using the logger
			logEntry.Info(scanner.Text())
			continue
		}
		logMap, ok := line.(map[string]interface{})
		if !ok {
			logEntry.Debugf("json log line not map[string]interface{}: %v", line)
			// fall back to using the logger
			logEntry.Info(scanner.Text())
			continue
		}

		for k, v := range logEntry.Data {
			logMap[k] = v
		}
		logLine, err := json.Marshal(logMap)
		if err != nil {
			logEntry.Debugf("marshal json log line: %v", err)
			// fall back to using the logger
			logEntry.Info(scanner.Text())
			continue
		}
		// Mark this log entry as one that is json that needs to be proxied
		logEntry := logEntry.WithField(app.ProxyJsonLogKey, true)
		// Log the line via the same centralized logger; the formatter should make sure it's "proxied"
		logEntry.Log(log.FatalLevel, string(logLine))
	}
}

func Output(cmd *exec.Cmd) (output []byte, err error) {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)
	output, err = cmd.Output()
	return
}

func MakeCommand(dir string, entrypoint string, args []string, envs []string) *exec.Cmd {
	cmd := exec.Command(entrypoint, args...)
	cmd.Env = append(cmd.Env, envs...)
	cmd.Dir = dir
	return cmd
}
