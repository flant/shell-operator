package debug

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	utils "github.com/flant/shell-operator/pkg/utils/file"
	structured_logger "github.com/flant/shell-operator/pkg/utils/structured-logger"
)

type Server struct {
	SocketPath string
	Prefix     string
	HttpPort   string

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

func (s *Server) WithHttpPort(port string) {
	s.HttpPort = port
}

func (s *Server) Init() (err error) {
	address := s.SocketPath

	err = os.MkdirAll(path.Dir(address), 0o700)
	if err != nil {
		log.Errorf("Debug HTTP server fail to create socket '%s': %v", address, err)
		return err
	}

	exists, err := utils.FileExists(address)
	if err != nil {
		log.Errorf("Debug HTTP server fail to check socket '%s': %v", address, err)
		return err
	}
	if exists {
		err = os.Remove(address)
		if err != nil {
			log.Errorf("Debug HTTP server fail to remove existing socket '%s': %v", address, err)
			return err
		}
	}

	// Check if socket is available
	listener, err := net.Listen("unix", address)
	if err != nil {
		log.Errorf("Debug HTTP server fail to listen on '%s': %v", address, err)
		return err
	}

	log.Infof("Debug endpoint listen on %s", address)

	s.Router = chi.NewRouter()
	s.Router.Use(structured_logger.NewStructuredLogger(log.StandardLogger(), "debugEndpoint"))
	s.Router.Use(middleware.Recoverer)

	go func() {
		if err := http.Serve(listener, s.Router); err != nil {
			log.Errorf("Error starting Debug socket server: %s", err)
			os.Exit(1)
		}
	}()

	if s.HttpPort != "" {
		port := fmt.Sprintf("127.0.0.1:%s", s.HttpPort)

		go func() {
			if err := http.ListenAndServe(port, s.Router); err != nil {
				log.Errorf("Error starting Debug HTTP server: %s", err)
				os.Exit(1)
			}
		}()
	}

	return nil
}

func (s *Server) Route(pattern string, handler func(request *http.Request) (interface{}, error)) {
	// Should not happen.
	if handler == nil {
		return
	}
	s.Router.Get(pattern, func(writer http.ResponseWriter, request *http.Request) {
		handleFormattedOutput(writer, request, handler)
	})
}

func (s *Server) RoutePOST(pattern string, handler func(request *http.Request) (interface{}, error)) {
	// Should not happen.
	if handler == nil {
		return
	}
	s.Router.Post(pattern, func(writer http.ResponseWriter, request *http.Request) {
		//
		err := request.ParseForm()
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(writer, "Error: %s", err)
			return
		}

		handleFormattedOutput(writer, request, handler)
	})
}

func handleFormattedOutput(writer http.ResponseWriter, request *http.Request, handler func(request *http.Request) (interface{}, error)) {
	out, err := handler(request)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(writer, "Error: %s", err)
		return
	}
	if out == nil && err == nil {
		return
	}

	format := FormatFromRequest(request)
	structured_logger.GetLogEntry(request).Debugf("use format '%s'", format)

	outBytes, err := transformUsingFormat(out, format)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(writer, "Error '%s' transform: %s", format, err)
		return
	}

	_, _ = writer.Write(outBytes)
}

func transformUsingFormat(val interface{}, format string) ([]byte, error) {
	var outBytes []byte
	var err error

	switch format {
	case "yaml":
		outBytes, err = yaml.Marshal(val)
	case "text":
		switch v := val.(type) {
		case string:
			outBytes = []byte(v)
		case fmt.Stringer:
			outBytes = []byte(v.String())
		case []byte:
			outBytes = v
		}
		if outBytes != nil {
			break
		}
		fallthrough
	case "json":
		fallthrough
	default:
		outBytes, err = json.Marshal(val)
	}

	return outBytes, err
}

func FormatFromRequest(request *http.Request) string {
	format := chi.URLParam(request, "format")
	if format == "" {
		format = "text"
	}
	return format
}
