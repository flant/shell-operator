package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/flant/shell-operator/pkg/unilogger"
	log "github.com/flant/shell-operator/pkg/unilogger"
	utils "github.com/flant/shell-operator/pkg/utils/labels"

	"github.com/flant/shell-operator/pkg/app"
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

func RunAndLogLines(cmd *exec.Cmd, logLabels map[string]string, logger *unilogger.Logger) (*CmdUsage, error) {
	// TODO observability
	stdErr := bytes.NewBuffer(nil)
	logEntry := utils.EnrichLoggerWithLabels(logger, logLabels)
	stdoutLogEntry := logEntry.With("output", "stdout")
	stderrLogEntry := logEntry.With("output", "stderr")

	logEntry.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	plo := &proxyLogger{app.LogProxyHookJSON, stdoutLogEntry, make([]byte, 0)}
	ple := &proxyLogger{app.LogProxyHookJSON, stderrLogEntry, make([]byte, 0)}
	cmd.Stdout = plo
	cmd.Stderr = io.MultiWriter(ple, stdErr)

	err := cmd.Run()
	if err != nil {
		if len(stdErr.Bytes()) > 0 {
			return nil, fmt.Errorf("%s", stdErr.String())
		}
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

type proxyLogger struct {
	logProxyHookJSON bool

	logger *unilogger.Logger

	buf []byte
}

func (pl *proxyLogger) Write(p []byte) (n int, err error) {
	if !pl.logProxyHookJSON {
		str := strings.TrimSpace(string(p))

		if str != "" {
			pl.logger.Log(context.Background(), unilogger.LevelInfo.Level(), str)
		}

		return len(p), nil
	}

	pl.buf = append(pl.buf, p...)

	var line interface{}
	err = json.Unmarshal(pl.buf, &line)
	if err != nil {
		if err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}

		return len(p), err
	}

	logMap, ok := line.(map[string]interface{})
	defer func() {
		pl.buf = []byte{}
	}()

	if !ok {
		pl.logger.Debugf("json log line not map[string]interface{}: %v", line)
		// fall back to using the logger
		pl.logger.Info(string(p))

		return len(p), err
	}

	logEntry := pl.logger.With(app.ProxyJsonLogKey, true)

	// logEntry.Log(log.FatalLevel, string(logLine))
	logEntry.Log(context.Background(), unilogger.LevelFatal.Level(), "hook result", slog.Any("hook", logMap))

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
