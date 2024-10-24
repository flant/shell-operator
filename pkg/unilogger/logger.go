package unilogger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/DataDog/gostackparse"
	logContext "github.com/flant/shell-operator/pkg/unilogger/context"
)

const KeyComponent = "component"

type logger = slog.Logger
type handlerOptions = *slog.HandlerOptions

type Logger struct {
	*logger

	addSourceVar *AddSourceVar
	level        *slog.LevelVar
	name         string

	slogHandler *SlogHandler
}

type Options struct {
	handlerOptions

	Level  slog.Level
	Output io.Writer

	TimeFunc func(t time.Time) time.Time
}

func NewNop() *Logger {
	return &Logger{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}
}

func NewLogger(opts Options) *Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}

	if opts.TimeFunc == nil {
		opts.TimeFunc = func(t time.Time) time.Time {
			return t
		}
	}

	l := &Logger{
		addSourceVar: new(AddSourceVar),
		level:        new(slog.LevelVar),
	}

	l.SetLevel(Level(opts.Level))

	// getting absolute binary path
	binaryPath := filepath.Dir(os.Args[0])
	// if it's go-build temporary folder
	if strings.Contains(binaryPath, "go-build") {
		binaryPath, _ = filepath.Abs("./../")
	}

	sourceFormat := "%s:%d"
	if os.Getenv("IDEA_DEVELOPMENT") != "" {
		sourceFormat = " %s:%d "
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     l.level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			// skip standard fields
			case slog.LevelKey, slog.MessageKey, slog.TimeKey:
				return slog.Attr{}
			case slog.SourceKey:
				if !*l.addSourceVar.Source() {
					return slog.Attr{}
				}

				s := a.Value.Any().(*slog.Source)

				a.Value = slog.StringValue(fmt.Sprintf(sourceFormat,
					// trim all folders before project root
					// trim first '/'
					strings.TrimPrefix(s.File, binaryPath)[1:],
					s.Line,
				))
			}

			return a
		},
	}

	l.slogHandler = NewHandler(opts.Output, handlerOpts, opts.TimeFunc)

	l.logger = slog.New(l.slogHandler.WithAttrs(nil))

	return l
}

func (l *Logger) GetLevel() Level {
	return Level(l.level.Level())
}

func (l *Logger) SetLevel(level Level) {
	l.addSourceVar.Set(slog.Level(level) <= slog.LevelDebug)

	l.level.Set(slog.Level(level))
}

func (l *Logger) SetOutput(w io.Writer) {
	l.slogHandler.w = w
}

func (l *Logger) Named(name string) *Logger {
	currName := name
	if l.name != "" {
		currName = fmt.Sprintf("%s.%s", l.name, name)
	}

	return &Logger{
		logger:       l.logger.With(slog.String("logger", currName)),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         currName,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:       l.logger.With(args...),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         l.name,
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		logger:       l.logger.WithGroup(name),
		addSourceVar: l.addSourceVar,
		level:        l.level,
		name:         l.name,
	}
}

func (l *Logger) Trace(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelTrace.Level(), msg, args...)
}

func (l *Logger) Tracef(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelTrace.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Logf(ctx context.Context, level Level, format string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	l.Log(ctx, level.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Debugf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelDebug.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Infof(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelInfo.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	l.Log(ctx, LevelWarn.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Error(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), msg, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelError.Level(), fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), msg, args...)

	os.Exit(1)
}

func (l *Logger) Fatalf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	l.Log(ctx, LevelFatal.Level(), fmt.Sprintf(format, args...))

	os.Exit(1)
}

func getStack() string {
	stack := debug.Stack()
	goroutines, _ := gostackparse.Parse(bytes.NewReader(stack))
	buf := bytes.NewBuffer([]byte{})
	json.NewEncoder(buf).Encode(goroutines)

	return strings.TrimSpace(buf.String())
}

func ParseLevel(rawLogLevel string) (Level, error) {
	switch strings.ToLower(rawLogLevel) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, errors.New("no level found")
	}
}

func LogLevelFromStr(rawLogLevel string) Level {
	switch strings.ToLower(rawLogLevel) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}
