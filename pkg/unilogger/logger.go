package unilogger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	logContext "github.com/flant/shell-operator/pkg/unilogger/context"
)

type logger = slog.Logger
type handlerOptions = *slog.HandlerOptions

type Logger struct {
	*logger

	addSource *bool
	level     *slog.Level
	name      string

	slogHandler *SlogHandler
}

type Options struct {
	handlerOptions

	AddSource bool
	Level     slog.Level
	Output    io.Writer
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

	l := &Logger{
		addSource: &opts.AddSource,
		level:     &opts.Level,
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     l.level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			// skip standard fields
			case slog.LevelKey, slog.MessageKey, slog.TimeKey:
				return slog.Attr{}
			case slog.SourceKey:
				if !*l.addSource {
					return slog.Attr{}
				}

				s := a.Value.Any().(*slog.Source)

				dir, file := filepath.Split(s.File)

				a.Value = slog.StringValue(fmt.Sprintf("%s:%d",
					filepath.Join(filepath.Base(dir), file),
					s.Line,
				))
			default:
				key := strings.SplitN(a.Key, ";", 1)
				if key[0] == "raw" {
					a.Key = strings.Join(key[1:], ";")
					a.Value = slog.StringValue(fmt.Sprintf("%#v", a.Value.Any()))
				}
			}

			return a
		},
	}

	l.slogHandler = NewHandler(opts.Output, handlerOpts)

	l.logger = slog.New(l.slogHandler.WithAttrs(nil))

	return l
}

func (l *Logger) SetLevel(level Level) {
	*l.addSource = slog.Level(level) <= slog.LevelDebug

	*l.level = slog.Level(level)
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
		logger:    l.logger.With(slog.String("logger", currName)),
		addSource: l.addSource,
		level:     l.level,
		name:      currName,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:    l.logger.With(args...),
		addSource: l.addSource,
		level:     l.level,
		name:      l.name,
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		logger:    l.logger.WithGroup(name),
		addSource: l.addSource,
		level:     l.level,
		name:      l.name,
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

func (l *Logger) Errorf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
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
	rawstack := string(debug.Stack())
	rawstack = strings.ReplaceAll(rawstack, "\t", "")
	split := strings.Split(rawstack, "\n")

	split = split[2:]

	return strings.Join(split, "")
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
