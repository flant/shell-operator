package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	if app.LogProxyHookJSON {
		plo := &proxyJSONLogger{make([]byte, 0), stdoutLogEntry}
		ple := &proxyJSONLogger{make([]byte, 0), stderrLogEntry}
		cmd.Stdout = plo
		cmd.Stderr = io.MultiWriter(ple, stdErr)
	} else {
		plo := &proxyLogger{make([]byte, 0), stdoutLogEntry}
		ple := &proxyLogger{make([]byte, 0), stderrLogEntry}
		cmd.Stdout = plo
		cmd.Stderr = io.MultiWriter(ple, stdErr)
	}

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

type proxyJSONLogger struct {
	buf []byte

	logger *unilogger.Logger
}

func (pj *proxyJSONLogger) Write(p []byte) (n int, err error) {
	pj.buf = append(pj.buf, p...)

	var line interface{}
	err = json.Unmarshal(pj.buf, &line)
	if err != nil {
		serr := new(json.SyntaxError)
		if errors.As(err, &serr) && err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}

		if err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}

		return len(p), err
	}

	logMap, ok := line.(map[string]interface{})
	if !ok {
		pj.logger.Debugf("json log line not map[string]interface{}: %v", line)
		// fall back to using the logger
		pj.logger.Info(string(p))

		return len(p), err
	}

	logEntry := pj.logger.With(app.ProxyJsonLogKey, true)

	// logEntry.Log(log.FatalLevel, string(logLine))
	logEntry.Log(context.Background(), unilogger.LevelFatal.Level(), "hook result", slog.Any("hook", logMap))

	return len(p), nil
}

type proxyLogger struct {
	buf []byte

	logger *unilogger.Logger
}

func (pl *proxyLogger) Write(p []byte) (n int, err error) {
	pl.buf = append(pl.buf, p...)

	pl.logger.Log(context.Background(), unilogger.LevelInfo.Level(), strings.TrimSpace(string(pl.buf)))

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
