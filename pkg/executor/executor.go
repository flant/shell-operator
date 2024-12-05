package executor

import (
	"bufio"
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

	"github.com/deckhouse/deckhouse/pkg/log"

	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

func Run(cmd *exec.Cmd) error {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	return cmd.Run()
}

type Executor struct {
	cmd              *exec.Cmd
	logProxyHookJSON bool
	proxyJsonKey     string
	logger           *log.Logger
}

func (e *Executor) WithLogProxyHookJSON(logProxyHookJSON bool) *Executor {
	e.logProxyHookJSON = logProxyHookJSON

	return e
}

func (e *Executor) WithLogProxyHookJSONKey(logProxyHookJSONKey string) *Executor {
	if logProxyHookJSONKey == "" {
		return e
	}

	e.proxyJsonKey = logProxyHookJSONKey

	return e
}

func (e *Executor) WithLogger(logger *log.Logger) *Executor {
	e.logger = logger

	return e
}

func (e *Executor) WithCMDStdout(w io.Writer) *Executor {
	e.cmd.Stdout = w

	return e
}

func (e *Executor) WithCMDStderr(w io.Writer) *Executor {
	e.cmd.Stderr = w

	return e
}

func NewExecutor(dir string, entrypoint string, args []string, envs []string) *Executor {
	cmd := exec.Command(entrypoint, args...)
	cmd.Env = append(cmd.Env, envs...)
	cmd.Dir = dir

	ex := &Executor{
		cmd:          cmd,
		proxyJsonKey: "proxyJsonLog",
		logger:       log.NewLogger(log.Options{}).Named("auto-executor"),
	}

	return ex
}

func (e *Executor) Output() ([]byte, error) {
	e.logger.Debugf("Executing command '%s' in '%s' dir", strings.Join(e.cmd.Args, " "), e.cmd.Dir)
	return e.cmd.Output()
}

type CmdUsage struct {
	Sys    time.Duration
	User   time.Duration
	MaxRss int64
}

func (e *Executor) RunAndLogLines(logLabels map[string]string) (*CmdUsage, error) {
	stdErr := bytes.NewBuffer(nil)
	logEntry := utils.EnrichLoggerWithLabels(e.logger, logLabels)
	stdoutLogEntry := logEntry.With("output", "stdout")
	stderrLogEntry := logEntry.With("output", "stderr")

	logEntry.Debugf("Executing command '%s' in '%s' dir", strings.Join(e.cmd.Args, " "), e.cmd.Dir)

	plo := &proxyLogger{e.logProxyHookJSON, e.proxyJsonKey, stdoutLogEntry, make([]byte, 0)}
	ple := &proxyLogger{e.logProxyHookJSON, e.proxyJsonKey, stderrLogEntry, make([]byte, 0)}
	e.cmd.Stdout = plo
	e.cmd.Stderr = io.MultiWriter(ple, stdErr)

	err := e.cmd.Run()
	if err != nil {
		if len(stdErr.Bytes()) > 0 {
			return nil, fmt.Errorf("stderr: %s", stdErr.String())
		}

		return nil, fmt.Errorf("cmd run: %w", err)
	}

	var usage *CmdUsage
	if e.cmd.ProcessState != nil {
		usage = &CmdUsage{
			Sys:  e.cmd.ProcessState.SystemTime(),
			User: e.cmd.ProcessState.UserTime(),
		}

		// FIXME Maxrss is Unix specific.
		sysUsage := e.cmd.ProcessState.SysUsage()
		if v, ok := sysUsage.(*syscall.Rusage); ok {
			// v.Maxrss is int32 on arm/v7
			usage.MaxRss = int64(v.Maxrss) //nolint:unconvert
		}
	}

	return usage, nil
}

type proxyLogger struct {
	logProxyHookJSON bool
	proxyJsonLogKey  string

	logger *log.Logger

	buf []byte
}

func (pl *proxyLogger) Write(p []byte) (int, error) {
	if !pl.logProxyHookJSON {
		pl.writerScanner(p)

		return len(p), nil
	}

	// join all parts of json
	pl.buf = append(pl.buf, p...)

	var line interface{}
	err := json.Unmarshal(pl.buf, &line)
	if err != nil {
		if err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}

		pl.logger.Debug("output is not json", log.Err(err))
		pl.writerScanner(p)

		return len(p), nil
	}

	logMap, ok := line.(map[string]interface{})
	defer func() {
		pl.buf = []byte{}
	}()

	if !ok {
		pl.logger.Debug("json log line not map[string]interface{}", slog.Any("line", line))

		// fall back to using the logger
		pl.logger.Info(string(p))

		return len(p), nil
	}

	// logEntry.Log(log.FatalLevel, string(logLine))
	pl.mergeAndLogInputLog(context.TODO(), logMap, "hook")

	return len(p), nil
}

func (pl *proxyLogger) writerScanner(p []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(p))

	// Set the buffer size to the maximum token size to avoid buffer overflows
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	// Define a split function to split the input into chunks of up to 64KB
	chunkSize := bufio.MaxScanTokenSize // 64KB
	splitFunc := func(data []byte, atEOF bool) (int, []byte, error) {
		if len(data) >= chunkSize {
			return chunkSize, data[:chunkSize], nil
		}

		return bufio.ScanLines(data, atEOF)
	}

	// Use the custom split function to split the input
	scanner.Split(splitFunc)

	// Scan the input and write it to the logger using the specified print function
	for scanner.Scan() {
		// prevent empty logging
		str := strings.TrimSpace(scanner.Text())
		if str == "" {
			continue
		}

		if len(str) > 10000 {
			str = fmt.Sprintf("%s:truncated", str[:10000])
		}

		pl.logger.Info(str)
	}

	// If there was an error while scanning the input, log an error
	if err := scanner.Err(); err != nil {
		pl.logger.Error("reading from scanner", log.Err(err))
	}
}

// level = level
// msg = msg
// prefix for all fields hook_
// source = hook_source
// stacktrace = hook_stacktrace
func (pl *proxyLogger) mergeAndLogInputLog(ctx context.Context, inputLog map[string]interface{}, prefix string) {
	var lvl log.Level

	lvlRaw, ok := inputLog[slog.LevelKey].(string)
	if ok {
		lvl = log.LogLevelFromStr(lvlRaw)
		delete(inputLog, slog.LevelKey)
	}

	msg, ok := inputLog[slog.MessageKey].(string)
	if !ok {
		msg = "hook result"
	}
	delete(inputLog, slog.MessageKey)
	delete(inputLog, slog.TimeKey)

	logLineRaw, _ := json.Marshal(inputLog)
	logLine := string(logLineRaw)

	logger := pl.logger.With(pl.proxyJsonLogKey, true)

	if len(logLine) > 10000 {
		logLine = fmt.Sprintf("%s:truncated", logLine[:10000])

		logger.Log(ctx, lvl.Level(), msg, slog.Any("hook", map[string]any{
			"truncated": logLine,
		}))

		return
	}

	for key, val := range inputLog {
		logger = logger.With(slog.Any(prefix+"_"+key, val))
	}

	logger.Log(ctx, lvl.Level(), msg)
}
