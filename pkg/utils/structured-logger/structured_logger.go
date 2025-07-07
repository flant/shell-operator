package structuredlogger

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5/middleware"
)

// StructuredLogger is a simple, but powerful implementation of a custom structured
// logger backed on slog. It is adapted from https://github.com/go-chi/chi
// 'logging' example.

func NewStructuredLogger(logger *log.Logger, componentLabel string) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{
		logger,
		componentLabel,
	})
}

type StructuredLogger struct {
	Logger         *log.Logger
	ComponentLabel string
}

func (l *StructuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &StructuredLoggerEntry{Logger: log.NewLogger()}

	entry.Logger = entry.Logger.With(
		// TODO: make snake_case
		slog.String("operator.component", l.ComponentLabel),
		slog.String("http_method", r.Method),
		slog.String("uri", r.RequestURI),
	)
	// entry.Logger.Info("request started")

	return entry
}

type StructuredLoggerEntry struct {
	Logger *log.Logger
}

func (l *StructuredLoggerEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	l.Logger = l.Logger.With(
		slog.Int("resp_status", status),
		slog.Int("resp_bytes_length", bytes),
		slog.Float64("resp_elapsed_ms", float64(elapsed.Truncate(10*time.Microsecond))/100.0),
	)

	l.Logger.Info("complete")
}

// This will log panics to log
func (l *StructuredLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger = l.Logger.With(
		slog.String("stack", string(stack)),
		slog.String("panic", fmt.Sprintf("%+v", v)),
	)
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func GetLogEntry(r *http.Request) *log.Logger {
	entry := middleware.GetLogEntry(r).(*StructuredLoggerEntry)
	return entry.Logger
}

// func LogEntrySetField(r *http.Request, key string, value interface{}) {
// 	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
// 		entry.Logger = entry.Logger.WithField(key, value)
// 	}
// }

// func LogEntrySetFields(r *http.Request, fields map[string]interface{}) {
// 	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
// 		entry.Logger = entry.Logger.WithFields(fields)
// 	}
// }
