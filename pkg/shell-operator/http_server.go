package shell_operator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	pkg "github.com/flant/shell-operator/pkg"
)

type baseHTTPServer struct {
	router chi.Router

	address string
	port    string

	mu     sync.Mutex
	srv    *http.Server
	doneCh chan struct{}
	logger *log.Logger
}

// Start runs the http server in a background goroutine. It returns
// immediately. Lifecycle errors (failure to listen, etc.) are reported
// asynchronously via the server logger; for graceful stop callers must use
// Shutdown(ctx). The ctx argument is retained for backwards compatibility but
// is not used for shutdown anymore — see Shutdown.
func (bhs *baseHTTPServer) Start(_ context.Context) {
	srv := &http.Server{
		Addr:         bhs.address + ":" + bhs.port,
		Handler:      bhs.router,
		ReadTimeout:  90 * time.Second,
		WriteTimeout: 90 * time.Second,
	}
	doneCh := make(chan struct{})

	bhs.mu.Lock()
	bhs.srv = srv
	bhs.doneCh = doneCh
	bhs.mu.Unlock()

	logger := bhs.serverLogger()

	go func() {
		defer close(doneCh)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("base http server stopped with error", log.Err(err))
		}
	}()

	logger.Info("base http server started",
		slog.String(pkg.LogKeyAddress, bhs.address),
		slog.String(pkg.LogKeyPort, bhs.port))
}

// Shutdown gracefully stops the http server. It is safe to call multiple times
// and when Start was never invoked — both return nil.
func (bhs *baseHTTPServer) Shutdown(ctx context.Context) error {
	bhs.mu.Lock()
	srv := bhs.srv
	doneCh := bhs.doneCh
	bhs.srv = nil
	bhs.mu.Unlock()

	if srv == nil {
		return nil
	}

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("base http server shutdown: %w", err)
	}
	if doneCh != nil {
		select {
		case <-doneCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	bhs.serverLogger().Info("base http server stopped")
	return nil
}

// serverLogger returns the per-instance logger if set, falling back to the
// package default so older constructors keep working.
func (bhs *baseHTTPServer) serverLogger() *log.Logger {
	bhs.mu.Lock()
	l := bhs.logger
	bhs.mu.Unlock()
	if l != nil {
		return l
	}
	return log.NewLogger()
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

func newBaseHTTPServer(address, port string, logger *log.Logger) *baseHTTPServer {
	router := chi.NewRouter()

	router.Mount("/debug", middleware.Profiler())

	router.Get("/discovery", func(writer http.ResponseWriter, _ *http.Request) {
		buf := bytes.NewBuffer(nil)
		walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
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
		logger:  logger,
	}

	return srv
}

func registerRootRoute(op *ShellOperator) {
	port := op.APIServer.port
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
</html>`, port)
	})
}
