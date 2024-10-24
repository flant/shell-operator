// global logger is deprecated
package unilogger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	logContext "github.com/flant/shell-operator/pkg/unilogger/context"
)

var defaultLogger atomic.Pointer[Logger]

func init() {
	defaultLogger.Store(NewLogger(Options{
		Level: slog.Level(LevelInfo),
	}))
}

func SetDefault(l *Logger) {
	defaultLogger.Store(l)
}

func SetDefaultLevel(l Level) {
	defaultLogger.Load().SetLevel(l)
}

func Default() *Logger { return defaultLogger.Load() }

func Log(ctx context.Context, level Level, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, level.Level(), msg, args...)
}

func Logf(ctx context.Context, level Level, format string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, level.Level(), fmt.Sprintf(format, args...))
}

func LogAttrs(ctx context.Context, level Level, msg string, attrs ...slog.Attr) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().LogAttrs(ctx, level.Level(), msg, attrs...)
}

func Trace(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelTrace.Level(), msg, args...)
}

func Tracef(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelTrace.Level(), fmt.Sprintf(format, args...))
}

func TraceContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelTrace.Level(), msg, args...)
}

func Debug(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelDebug.Level(), msg, args...)
}

func Debugf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelDebug.Level(), fmt.Sprintf(format, args...))
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelDebug.Level(), msg, args...)
}

func Info(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelInfo.Level(), msg, args...)
}

func Infof(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelInfo.Level(), fmt.Sprintf(format, args...))
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelInfo.Level(), msg, args...)
}

func Warn(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelWarn.Level(), msg, args...)
}

func Warnf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	Default().Log(ctx, LevelWarn.Level(), fmt.Sprintf(format, args...))
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelWarn.Level(), msg, args...)
}

func Error(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())
	Default().Log(ctx, LevelError.Level(), msg, args...)
}

func Errorf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())
	Default().Log(ctx, LevelError.Level(), fmt.Sprintf(format, args...))
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	Default().Log(ctx, LevelError.Level(), msg, args...)
}

func Fatal(msg string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), msg, args...)

	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	ctx := logContext.SetCustomKeyContext(context.Background())
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), fmt.Sprintf(format, args...))

	os.Exit(1)
}

func FatalContext(ctx context.Context, msg string, args ...any) {
	ctx = logContext.SetCustomKeyContext(ctx)
	ctx = logContext.SetStackTraceContext(ctx, getStack())

	Default().Log(ctx, LevelFatal.Level(), msg, args...)
	os.Exit(1)
}
