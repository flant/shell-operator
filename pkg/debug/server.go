package debug

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/utils/structured-logger"
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
			log.Errorf("Error starting Debug HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}

func (s *Server) Route(pattern string, handler func(request *http.Request) (interface{}, error)) {
	// Should not happen.
	if handler == nil {
		return
	}
	s.Router.Get(pattern, func(writer http.ResponseWriter, request *http.Request) {
		out, err := handler(request)

		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(writer, "Error: %s", err)
			return
		}

		format := chi.URLParam(request, "format")
		if format == "" {
			format = "text"
		}
		structured_logger.GetLogEntry(request).Debugf("use format '%s'", format)

		outBytes, err := TransformUsingFormat(out, format)

		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(writer, "Error '%s' transform: %s", format, err)
			return
		}

		_, _ = writer.Write(outBytes)
	})
}

func TransformUsingFormat(val interface{}, format string) ([]byte, error) {
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
