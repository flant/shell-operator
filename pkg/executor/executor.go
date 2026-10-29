package executor

import (
	"bufio"
	"bytes"
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"go.opentelemetry.io/otel"

	pkg "github.com/flant/shell-operator/pkg"
	json "github.com/flant/shell-operator/pkg/utils/json"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

const (
	serviceName = "executor"
)

// Run starts the command, waits for it to complete, and returns the error.
// The child PID is registered in the global process registry while the process
// is running so that a PID-1 zombie reaper does not steal it.
func Run(cmd *exec.Cmd) error {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debug("Executing command", slog.String(pkg.LogKeyCommand, strings.Join(cmd.Args, " ")), slog.String(pkg.LogKeyDir, cmd.Dir))

	if err := startAndRegister(cmd); err != nil {
		return err
	}
	defer unregisterPID(cmd.Process.Pid)

	return cmd.Wait()
}

// StderrError is returned by RunAndLogLines when a command fails and produces
// output on stderr. Callers can use errors.As to access the raw stderr content.
type StderrError struct {
	Message string
}

func (e *StderrError) Error() string {
	return e.Message
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

func (e *Executor) WithChroot(path string) *Executor {
	if len(path) > 0 {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{
			Chroot: path,
		}
		e.cmd.Path = strings.TrimPrefix(e.cmd.Path, path)
		e.cmd.Dir = "/"
	}

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
		logger:       log.NewLogger().Named("auto-executor"),
	}

	return ex
}

func (e *Executor) Output() ([]byte, error) {
	e.logger.Debug("Executing command",
		slog.String(pkg.LogKeyCommand, strings.Join(e.cmd.Args, " ")),
		slog.String(pkg.LogKeyDir, e.cmd.Dir))

	// Reproduce cmd.Output() but interleave PID registration so that the
	// PID-1 zombie reaper skips this process.
	if e.cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	e.cmd.Stdout = &stdout

	captureErr := e.cmd.Stderr == nil
	var stderrBuf bytes.Buffer
	if captureErr {
		e.cmd.Stderr = &stderrBuf
	}

	if err := startAndRegister(e.cmd); err != nil {
		return nil, err
	}
	defer unregisterPID(e.cmd.Process.Pid)

	err := e.cmd.Wait()
	if err != nil && captureErr {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderrBuf.Bytes()
		}
	}

	return stdout.Bytes(), err
}

type CmdUsage struct {
	Sys    time.Duration
	User   time.Duration
	MaxRss int64
}

func (e *Executor) RunAndLogLines(ctx context.Context, logLabels map[string]string) (*CmdUsage, error) {
	ctx, span := otel.Tracer(serviceName).Start(ctx, "RunAndLogLines")
	defer span.End()

	stdErr := bytes.NewBuffer(nil)
	logEntry := utils.EnrichLoggerWithLabels(e.logger, logLabels)
	stdoutLogEntry := logEntry.With(pkg.LogKeyOutput, "stdout")
	stderrLogEntry := logEntry.With(pkg.LogKeyOutput, "stderr")

	log.Debug("Executing command",
		slog.String(pkg.LogKeyCommand, strings.Join(e.cmd.Args, " ")),
		slog.String(pkg.LogKeyDir, e.cmd.Dir))

	plo := &proxyLogger{
		ctx:              ctx,
		logProxyHookJSON: e.logProxyHookJSON,
		proxyJsonLogKey:  e.proxyJsonKey,
		logger:           stdoutLogEntry,
		buf:              make([]byte, 0),
	}

	ple := &proxyLogger{
		ctx:              ctx,
		logProxyHookJSON: e.logProxyHookJSON,
		proxyJsonLogKey:  e.proxyJsonKey,
		logger:           stderrLogEntry,
		buf:              make([]byte, 0),
	}

	e.cmd.Stdout = plo
	e.cmd.Stderr = io.MultiWriter(ple, stdErr)

	if err := startAndRegister(e.cmd); err != nil {
		return nil, fmt.Errorf("cmd start: %w", err)
	}
	defer unregisterPID(e.cmd.Process.Pid)

	err := e.cmd.Wait()
	plo.flush()
	ple.flush()
	if err != nil {
		if len(stdErr.Bytes()) > 0 {
			return nil, &StderrError{Message: stdErr.String()}
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
	ctx context.Context

	logProxyHookJSON bool
	proxyJsonLogKey  string
	logger           *log.Logger
	buf              []byte
	jsonBuf          []byte
}

func (pl *proxyLogger) Write(p []byte) (int, error) {
	pl.buf = append(pl.buf, p...)

	for len(pl.buf) > 0 {
		lineSize := bytes.IndexByte(pl.buf, '\n') + 1
		if !pl.hasPendingJSON() && len(pl.buf) >= bufio.MaxScanTokenSize && (lineSize == 0 || lineSize > bufio.MaxScanTokenSize) {
			lineSize = bufio.MaxScanTokenSize
		}
		if lineSize == 0 {
			break
		}

		pl.logLine(pl.buf[:lineSize])
		pl.buf = pl.buf[lineSize:]
	}
	if len(pl.buf) == 0 {
		pl.buf = nil
	}

	return len(p), nil
}

func (pl *proxyLogger) hasPendingJSON() bool {
	if len(pl.jsonBuf) > 0 {
		return true
	}
	line := bytes.TrimLeft(pl.buf, " \t\r\n")
	return bytes.HasPrefix(line, []byte("{")) || (pl.logProxyHookJSON && (bytes.HasPrefix(line, []byte("[")) || bytes.HasPrefix(line, []byte("\""))))
}

// flush logs the final unterminated record after the output stream ends.
func (pl *proxyLogger) flush() {
	if len(pl.buf) > 0 {
		pl.logLine(pl.buf)
		pl.buf = nil
	}
	if len(pl.jsonBuf) > 0 {
		pl.logText(string(pl.jsonBuf))
		pl.jsonBuf = nil
	}
}

func (pl *proxyLogger) logLine(raw []byte) {
	line := strings.TrimSpace(string(raw))
	if line == "" && len(pl.jsonBuf) == 0 {
		return
	}

	// JSON records can span both physical lines and separate Write calls.
	if len(pl.jsonBuf) > 0 || strings.HasPrefix(line, "{") || (pl.logProxyHookJSON && (strings.HasPrefix(line, "[") || strings.HasPrefix(line, "\""))) {
		pl.jsonBuf = append(pl.jsonBuf, raw...)
		err := pl.tryLogJSON(string(pl.jsonBuf))
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return
		}
		if err != nil {
			pl.logText(string(pl.jsonBuf))
		}
		pl.jsonBuf = nil
		return
	}

	if err := pl.tryLogJSON(string(raw)); err == nil {
		return
	}
	pl.logText(line)
}

func (pl *proxyLogger) logText(line string) {
	line = strings.TrimSpace(line)
	if len(line) > 10000 {
		line = fmt.Sprintf("%s:truncated", line[:10000])
	}
	pl.logger.Info(line)
}

// tryLogJSON logs a complete JSON record with the hook's structured fields.
func (pl *proxyLogger) tryLogJSON(s string) error {
	var value interface{}
	if err := json.Unmarshal([]byte(s), &value); err != nil {
		// go-json reports some incomplete documents as syntax errors.
		var raw stdjson.RawMessage
		if decodeErr := stdjson.NewDecoder(strings.NewReader(s)).Decode(&raw); errors.Is(decodeErr, io.ErrUnexpectedEOF) {
			return io.ErrUnexpectedEOF
		}
		return err
	}

	logMap, ok := value.(map[string]interface{})
	if ok || (value == nil && !pl.logProxyHookJSON) {
		pl.mergeAndLogInputLog(pl.ctx, logMap, "hook")
		return nil
	}
	if !pl.logProxyHookJSON {
		return errors.New("json log line is not an object")
	}

	pl.logger.Debug("json log line not map[string]interface{}", slog.Any(pkg.LogKeyLine, value))
	pl.logger.Info(s)
	return nil
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

		logger.Log(ctx, lvl.Level(), msg, slog.Any(pkg.LogKeyHook, map[string]any{
			"truncated": logLine,
		}))

		return
	}

	for key, val := range inputLog {
		logger = logger.With(slog.Any(prefix+"_"+key, val))
	}

	logger.Log(ctx, lvl.Level(), msg)
}
