package debug

import (
	"net"
	"net/http"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

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
	s.Router.Use(structured_logger.NewStructuredLogger(log.StandardLogger()))
	s.Router.Use(middleware.Recoverer)

	go func() {
		if err := http.Serve(listener, s.Router); err != nil {
			log.Errorf("Error starting Debug HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}
