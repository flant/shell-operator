package shell_operator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

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
		ReadTimeout:  90 * time.Second,
		WriteTimeout: 90 * time.Second,
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

	// inject pprof
	router.Mount("/debug", middleware.Profiler())

	router.Get("/discovery", func(writer http.ResponseWriter, _ *http.Request) {
		buf := bytes.NewBuffer(nil)
		walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			// skip pprof routes
			if strings.HasPrefix(route, "/debug/") {
				return nil
			}
			_, _ = fmt.Fprintf(buf, "%s %s\n", method, route)
			return nil
		}

		err := chi.Walk(router, walkFn)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		buf.WriteString("GET /debug/pprof/*\n")

		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(buf.Bytes())
	})

	srv := &baseHTTPServer{
		router:  router,
		address: address,
		port:    port,
	}

	return srv
}

func registerRootRoute(op *ShellOperator) {
	op.APIServer.RegisterRoute(http.MethodGet, "/", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html>
  <head><title>Shell operator</title></head>
  <body>
    <h1>Shell operator</h1>
    <dl>
      <dt>Show all possible routes</dt>
      <dd>- curl http://SHELL_OPERATOR_IP:%[1]s/discovery</dd>
      <br>
      <dt>Run golang profiling</dt>
      <dd>- go tool pprof http://SHELL_OPERATOR_IP:%[1]s/debug/pprof/profile</dd>
    </dl>
  </body>
</html>`, app.ListenPort)
	})
}
