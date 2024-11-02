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

func RunAndLogLines(cmd *exec.Cmd, logLabels map[string]string, logger *log.Logger) (*CmdUsage, error) {
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

		return len(p), err
	}

	logMap, ok := line.(map[string]interface{})
	defer func() {
		pl.buf = []byte{}
	}()

	if !ok {
		pl.logger.Debug("json log line not map[string]interface{}", slog.Any("line", line))

		// fall back to using the logger
		pl.logger.Info(string(p))

		return len(p), err
	}

	logger := pl.logger.With(app.ProxyJsonLogKey, true)

	logLineRaw, _ := json.Marshal(logMap)
	logLine := string(logLineRaw)

	if len(logLine) > 10000 {
		logLine = fmt.Sprintf("%s:truncated", string(logLine[:10000]))

		logger.Log(context.Background(), log.LevelFatal.Level(), "hook result", slog.Any("hook", map[string]any{
			"truncated": logLine,
		}))

		return len(p), nil
	}

	// logEntry.Log(log.FatalLevel, string(logLine))
	logger.Log(context.Background(), log.LevelFatal.Level(), "hook result", slog.Any("hook", logMap))

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
		pl.logger.Error("reading from scanner", slog.String("error", err.Error()))
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
