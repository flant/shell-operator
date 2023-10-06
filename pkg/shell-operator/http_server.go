package shell_operator

import (
	"fmt"
	"net"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
)

func startHttpServer(ip string, port string, mux *http.ServeMux) error {
	address := fmt.Sprintf("%s:%s", ip, port)

	// Check if port is available
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on '%s' fails: %v", address, err)
	}

	log.Infof("Listen on %s", address)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Errorf("Fatal: error starting HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}

func registerDefaultRoutes(op *ShellOperator) {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html>
    <head><title>Shell operator</title></head>
    <body>
    <h1>Shell operator</h1>
    <pre>go tool pprof goprofex http://&lt;SHELL_OPERATOR_IP&gt;:%s/debug/pprof/profile</pre>
    </body>
    </html>`, app.ListenPort)
	})

	http.HandleFunc("/metrics", func(writer http.ResponseWriter, request *http.Request) {
		if op.MetricStorage != nil {
			op.MetricStorage.Handler().ServeHTTP(writer, request)
		}
	})
}
