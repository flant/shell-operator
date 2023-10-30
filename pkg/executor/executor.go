package executor

import (
	"encoding/json"
	"github.com/flant/shell-operator/internal/app"
	"os/exec"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

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

	if app.LogProxyHookJSON {
		plo := &proxyJSONLogger{stdoutLogEntry, make([]byte, 0)}
		ple := &proxyJSONLogger{stderrLogEntry, make([]byte, 0)}
		cmd.Stdout = plo
		cmd.Stderr = ple
	} else {
		cmd.Stdout = stdoutLogEntry.Writer()
		cmd.Stderr = stderrLogEntry.Writer()
	}

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

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
			usage.MaxRss = int64(v.Maxrss) //nolint:unconvert
		}
	}

	return usage, err
}

type proxyJSONLogger struct {
	*log.Entry

	buf []byte
}

func (pj *proxyJSONLogger) Write(p []byte) (n int, err error) {
	pj.buf = append(pj.buf, p...)

	var line interface{}
	err = json.Unmarshal(pj.buf, &line)
	if err != nil {
		if err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}
		return len(p), err
	}

	logMap, ok := line.(map[string]interface{})
	if !ok {
		pj.Debugf("json log line not map[string]interface{}: %v", line)
		// fall back to using the logger
		pj.Info(string(p))
		return len(p), err
	}

	for k, v := range pj.Data {
		logMap[k] = v
	}

	logLine, _ := json.Marshal(logMap)

	logEntry := pj.WithField(app.ProxyJsonLogKey, true)

	logEntry.Log(log.FatalLevel, string(logLine))

	return len(p), nil
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
