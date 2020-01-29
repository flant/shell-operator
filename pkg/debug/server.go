package debug

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/file"
)

type Server struct {
	SocketPath string
	Prefix     string

	Router chi.Router
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) WithSocketPath(path string) {
	s.SocketPath = path
}

func (s *Server) WithPrefix(prefix string) {
	s.Prefix = prefix
}

func (s *Server) Init() (err error) {
	address := app.DebugUnixSocket

	err = os.MkdirAll(path.Dir(address), 0700)
	if err != nil {
		logrus.Error("Debug HTTP server fail to create socket '%s': %v", address, err)
		return err
	}

	exists, err := utils.FileExists(address)
	if err != nil {
		logrus.Error("Debug HTTP server fail to check socket '%s': %v", address, err)
		return err
	}
	if exists {
		err = os.Remove(address)
		if err != nil {
			logrus.Error("Debug HTTP server fail to remove existing socket '%s': %v", address, err)
			return err
		}
	}

	// Check if socket is available
	listener, err := net.Listen("unix", address)
	if err != nil {
		logrus.Error("Debug HTTP server fail to listen on '%s': %v", address, err)
		return err
	}

	logrus.Infof("Debug endpoint listen on %s", address)

	s.Router = chi.NewRouter()
	s.Router.Use(NewStructuredLogger(logrus.StandardLogger()))
	s.Router.Use(middleware.Recoverer)

	go func() {
		if err := http.Serve(listener, s.Router); err != nil {
			logrus.Errorf("Error starting Debug HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}

// StructuredLogger is a simple, but powerful implementation of a custom structured
// logger backed on logrus. It is adapted from https://github.com/go-chi/chi
// 'logging' example.

func NewStructuredLogger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{logger})
}

type StructuredLogger struct {
	Logger *logrus.Logger
}

func (l *StructuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &StructuredLoggerEntry{Logger: logrus.NewEntry(l.Logger)}
	logFields := logrus.Fields{}
	logFields["operator.component"] = "debugEndpoint"
	logFields["http_method"] = r.Method
	logFields["uri"] = r.RequestURI

	entry.Logger = entry.Logger.WithFields(logFields)
	//entry.Logger.Infoln("request started")

	return entry
}

type StructuredLoggerEntry struct {
	Logger logrus.FieldLogger
}

func (l *StructuredLoggerEntry) Write(status, bytes int, elapsed time.Duration) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"resp_status":       status,
		"resp_bytes_length": bytes,
		"resp_elapsed_ms":   float64(elapsed.Truncate(10*time.Microsecond)) / 100.0,
	})

	l.Logger.Infoln("complete")
}

// This will log panics to log
func (l *StructuredLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"stack": string(stack),
		"panic": fmt.Sprintf("%+v", v),
	})
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func GetLogEntry(r *http.Request) logrus.FieldLogger {
	entry := middleware.GetLogEntry(r).(*StructuredLoggerEntry)
	return entry.Logger
}

func LogEntrySetField(r *http.Request, key string, value interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithField(key, value)
	}
}

func LogEntrySetFields(r *http.Request, fields map[string]interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithFields(fields)
	}
}
