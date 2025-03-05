package debug

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/pkg/log"

	utils "github.com/flant/shell-operator/pkg/utils/file"
	structuredLogger "github.com/flant/shell-operator/pkg/utils/structured-logger"
)

type Server struct {
	Prefix     string
	SocketPath string
	HttpAddr   string

	Router chi.Router

	logger *log.Logger
}

func NewServer(prefix, socketPath, httpAddr string, logger *log.Logger) *Server {
	router := chi.NewRouter()

	router.Use(structuredLogger.NewStructuredLogger(logger.Named("debugEndpoint"), "debugEndpoint"))
	router.Use(middleware.Recoverer)

	return &Server{
		Prefix:     prefix,
		SocketPath: socketPath,
		HttpAddr:   httpAddr,
		Router:     router,
		logger:     logger,
	}
}

func (s *Server) Init() error {
	address := s.SocketPath

	if err := os.MkdirAll(path.Dir(address), 0o700); err != nil {
		return fmt.Errorf("Debug HTTP server fail to create socket '%s': %w", address, err)
	}

	exists, err := utils.FileExists(address)
	if err != nil {
		return fmt.Errorf("Debug HTTP server fail to check socket '%s': %w", address, err)
	}

	if exists {
		if err := os.Remove(address); err != nil {
			return fmt.Errorf("Debug HTTP server fail to check socket '%s': %w", address, err)
		}
	}

	// Check if socket is available
	listener, err := net.Listen("unix", address)
	if err != nil {
		return fmt.Errorf("Debug HTTP server fail to listen on '%s': %w", address, err)
	}

	s.logger.Info("Debug endpoint listen on address", slog.String("address", address))

	go func() {
		if err := http.Serve(listener, s.Router); err != nil {
			s.logger.Error("Error starting Debug socket server", log.Err(err))
			os.Exit(1)
		}
	}()

	if s.HttpAddr != "" {
		go func() {
			if err := http.ListenAndServe(s.HttpAddr, s.Router); err != nil {
				s.logger.Error("Error starting Debug HTTP server", log.Err(err))
				os.Exit(1)
			}
		}()
	}

	return nil
}

// RegisterHandler registers http handler for unix/http debug server
func (s *Server) RegisterHandler(method, pattern string, handler func(request *http.Request) (interface{}, error)) {
	if method == "" {
		return
	}
	if pattern == "" {
		return
	}
	if handler == nil {
		return
	}

	switch method {
	case http.MethodGet:
		s.Router.Get(pattern, func(writer http.ResponseWriter, request *http.Request) {
			handleFormattedOutput(writer, request, handler)
		})

	case http.MethodPost:
		s.Router.Post(pattern, func(writer http.ResponseWriter, request *http.Request) {
			handleFormattedOutput(writer, request, handler)
		})
	}
}

func handleFormattedOutput(writer http.ResponseWriter, request *http.Request, handler func(request *http.Request) (interface{}, error)) {
	out, err := handler(request)
	if err != nil {
		if _, ok := err.(*BadRequestError); ok {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if out == nil {
		writer.WriteHeader(http.StatusOK)
		return
	}

	format := FormatFromRequest(request)
	structuredLogger.GetLogEntry(request).Debug("used format", slog.String("format", format))

	switch format {
	case "text":
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "json":
		writer.Header().Set("Content-Type", "application/json")
	case "yaml":
		writer.Header().Set("Content-Type", "application/yaml")
	case "png":
		writer.Header().Set("Content-Type", "image/png")
	}
	writer.WriteHeader(http.StatusOK)

	err = transformUsingFormat(writer, out, format)
	if err != nil {
		http.Error(writer, fmt.Sprintf("Error '%s' transform: %s", format, err), http.StatusInternalServerError)
	}
}

func transformUsingFormat(w io.Writer, val interface{}, format string) error {
	var err error

	switch format {
	case "png":
		switch v := val.(type) {
		case []byte:
			_, err = w.Write(v)
		default:
			err = fmt.Errorf("unsupported type %T", val)
		}
	case "yaml":
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		err = enc.Encode(val)
	case "text":
		switch v := val.(type) {
		case string:
			_, err = w.Write([]byte(v))
		case fmt.Stringer:
			_, err = w.Write([]byte(v.String()))
		case []byte:
			_, err = w.Write(v)
		default:
			err = json.NewEncoder(w).Encode(val)
		}
		if err == nil {
			break
		}
		fallthrough
	case "json":
		fallthrough
	default:
		err = json.NewEncoder(w).Encode(val)
	}

	return err
}

func FormatFromRequest(request *http.Request) string {
	format := chi.URLParam(request, "format")
	fmt.Println("fetch format", format)
	if format == "" {
		format = "text"
	}
	return format
}

type BadRequestError struct {
	Msg string
}

func (be *BadRequestError) Error() string {
	return be.Msg
}
