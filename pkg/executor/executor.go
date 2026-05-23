package executor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
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

// ProcessRegistry tracks PIDs of processes started by the executor so that
// a PID-1 zombie reaper can skip them (their parent already calls wait).
// This prevents the reaper from stealing a child that cmd.Wait expects to reap.
type ProcessRegistry struct {
	mu         sync.RWMutex
	activePIDs map[int32]struct{}
}

// NewProcessRegistry creates a new ProcessRegistry.
func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{
		activePIDs: make(map[int32]struct{}),
	}
}

// Register adds pid to the set of active PIDs.
func (r *ProcessRegistry) Register(pid int) {
	r.mu.Lock()
	r.activePIDs[int32(pid)] = struct{}{}
	r.mu.Unlock()
}

// Unregister removes pid from the set of active PIDs.
func (r *ProcessRegistry) Unregister(pid int) {
	r.mu.Lock()
	delete(r.activePIDs, int32(pid))
	r.mu.Unlock()
}

// IsActive reports whether pid is currently tracked as an active process.
func (r *ProcessRegistry) IsActive(pid int) bool {
	r.mu.RLock()
	_, ok := r.activePIDs[int32(pid)]
	r.mu.RUnlock()
	return ok
}

// Registry is the global process registry shared between the executor and
// the PID-1 zombie reaper. All executor methods that spawn child processes
// register their PIDs here so the reaper can skip them.
var Registry = NewProcessRegistry()

// Run starts the command, waits for it to complete, and returns the error.
// The child PID is registered in the global Registry while the process is
// running so that a PID-1 zombie reaper does not steal it.
func Run(cmd *exec.Cmd) error {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debug("Executing command", slog.String(pkg.LogKeyCommand, strings.Join(cmd.Args, " ")), slog.String(pkg.LogKeyDir, cmd.Dir))

	if err := cmd.Start(); err != nil {
		return err
	}

	Registry.Register(cmd.Process.Pid)
	defer Registry.Unregister(cmd.Process.Pid)

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

	if err := e.cmd.Start(); err != nil {
		return nil, err
	}

	Registry.Register(e.cmd.Process.Pid)
	defer Registry.Unregister(e.cmd.Process.Pid)

	time.Sleep(2 * time.Second)
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

	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("cmd start: %w", err)
	}

	Registry.Register(e.cmd.Process.Pid)
	defer Registry.Unregister(e.cmd.Process.Pid)

	err := e.cmd.Wait()
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
		pl.buf = pl.buf[:0]
	}()

	if !ok {
		pl.logger.Debug("json log line not map[string]interface{}", slog.Any(pkg.LogKeyLine, line))

		// fall back to using the logger
		pl.logger.Info(string(p))

		return len(p), nil
	}

	pl.mergeAndLogInputLog(pl.ctx, logMap, "hook")

	return len(p), nil
}

func (pl *proxyLogger) writerScanner(p []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	var jsonBuf []string
	// Split large entries into chunks
	chunkSize := bufio.MaxScanTokenSize // 64KB
	splitFunc := func(data []byte, atEOF bool) (int, []byte, error) {
		if len(data) >= chunkSize {
			return chunkSize, data[:chunkSize], nil
		}
		return bufio.ScanLines(data, atEOF)
	}
	scanner.Split(splitFunc)
	// Scan the input and write it to the logger using the specified print function
	for scanner.Scan() {
		// Prevent empty logging
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if len(jsonBuf) > 0 || strings.HasPrefix(line, "{") {
			jsonBuf = append(jsonBuf, line)
			joined := strings.Join(jsonBuf, "\n")
			if err := pl.tryLogJSON(joined); err == nil {
				jsonBuf = nil
			}
			continue
		}

		if len(line) > 10000 {
			line = fmt.Sprintf("%s:truncated", line[:10000])
		}

		if err := pl.tryLogJSON(line); err == nil {
			continue
		}
		pl.logger.Info(line)
	}
	// If there was an error while scanning the input, log an error
	if err := scanner.Err(); err != nil {
		pl.logger.Error("reading from scanner", log.Err(err))
	}
}

// tryLogJSON tries to parse the string as JSON and log it if it succeeds
func (pl *proxyLogger) tryLogJSON(s string) error {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(s), &m)
	if err == nil {
		pl.mergeAndLogInputLog(pl.ctx, m, "hook")
		return nil
	}
	return fmt.Errorf("failed to parse json: %w", err)
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
