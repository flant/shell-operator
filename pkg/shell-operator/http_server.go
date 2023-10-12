package shell_operator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
)

type baseHTTPServer struct {
	router chi.Router

	address string
	port    string
}

// Start runs http server
func (bhs *baseHTTPServer) Start(ctx context.Context) {
	srv := &http.Server{
		Addr:         bhs.address + ":" + bhs.port,
		Handler:      bhs.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("base http server listen: %s\n", err)
		}
	}()
	log.Infof("base http server started at %s:%s", bhs.address, bhs.port)

	go func() {
		<-ctx.Done()
		log.Info("base http server stopped")

		cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer func() {
			// extra handling here
			cancel()
		}()

		if err := srv.Shutdown(cctx); err != nil {
			log.Fatalf("base http server shutdown failed:%+v", err)
		}
	}()
}

// RegisterRoute register http.HandlerFunc
func (bhs *baseHTTPServer) RegisterRoute(method, pattern string, h http.HandlerFunc) {
	switch method {
	case http.MethodGet:
		bhs.router.Get(pattern, h)

	case http.MethodPost:
		bhs.router.Post(pattern, h)

	case http.MethodPut:
		bhs.router.Put(pattern, h)

	case http.MethodDelete:
		bhs.router.Delete(pattern, h)
	}
}

func newBaseHTTPServer(address, port string) *baseHTTPServer {
	router := chi.NewRouter()

	srv := &baseHTTPServer{
		router:  router,
		address: address,
		port:    port,
	}

	return srv
}

func registerDefaultRoutes(op *ShellOperator) {
	op.APIServer.RegisterRoute(http.MethodGet, "/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html>
    <head><title>Shell operator</title></head>
    <body>
    <h1>Shell operator</h1>
    <pre>go tool pprof goprofex http://&lt;SHELL_OPERATOR_IP&gt;:%s/debug/pprof/profile</pre>
    </body>
    </html>`, app.ListenPort)
	})

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", func(writer http.ResponseWriter, request *http.Request) {
		if op.MetricStorage != nil {
			op.MetricStorage.Handler().ServeHTTP(writer, request)
		}
	})
}
