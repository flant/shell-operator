package debug

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/go_lib/log"
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

func (s *Server) Init() (err error) {
	address := s.SocketPath

	err = os.MkdirAll(path.Dir(address), 0o700)
	if err != nil {
		s.logger.Errorf("Debug HTTP server fail to create socket '%s': %v", address, err)
		return err
	}

	exists, err := utils.FileExists(address)
	if err != nil {
		s.logger.Errorf("Debug HTTP server fail to check socket '%s': %v", address, err)
		return err
	}
	if exists {
		err = os.Remove(address)
		if err != nil {
			s.logger.Errorf("Debug HTTP server fail to remove existing socket '%s': %v", address, err)
			return err
		}
	}

	// Check if socket is available
	listener, err := net.Listen("unix", address)
	if err != nil {
		s.logger.Errorf("Debug HTTP server fail to listen on '%s': %v", address, err)
		return err
	}

	s.logger.Infof("Debug endpoint listen on %s", address)

	go func() {
		if err := http.Serve(listener, s.Router); err != nil {
			s.logger.Errorf("Error starting Debug socket server: %s", err)
			os.Exit(1)
		}
	}()

	if s.HttpAddr != "" {
		go func() {
			if err := http.ListenAndServe(s.HttpAddr, s.Router); err != nil {
				s.logger.Errorf("Error starting Debug HTTP server: %s", err)
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
	structuredLogger.GetLogEntry(request).Debugf("use format '%s'", format)

	switch format {
	case "text":
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "json":
		writer.Header().Set("Content-Type", "application/json")
	case "yaml":
		writer.Header().Set("Content-Type", "application/yaml")
	}
	writer.WriteHeader(http.StatusOK)

	err = transformUsingFormat(writer, out, format)
	if err != nil {
		http.Error(writer, fmt.Sprintf("Error '%s' transform: %s", format, err), http.StatusInternalServerError)
	}
}

func transformUsingFormat(w io.Writer, val interface{}, format string) (err error) {
	switch format {
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
