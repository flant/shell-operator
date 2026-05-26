package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"

	pkg "github.com/flant/shell-operator/pkg"
)

type WebhookServer struct {
	Settings  *Settings
	Namespace string
	Router    chi.Router
	Logger    *log.Logger

	mu       sync.Mutex
	srv      *http.Server
	listener net.Listener
	doneCh   chan struct{}
}

func NewWebhookServer(settings *Settings, namespace string, router chi.Router, logger *log.Logger) *WebhookServer {
	return &WebhookServer{
		Settings:  settings,
		Namespace: namespace,
		Router:    router,
		Logger:    logger,
	}
}

// Start runs https server to listen for AdmissionReview requests from the API-server.
// Start does not block; the server runs in its own goroutine. A non-nil error
// means the server failed to bind or load TLS material. Once Start returns nil,
// the only way the server stops is via Shutdown.
func (s *WebhookServer) Start() error {
	keyPair, err := tls.LoadX509KeyPair(
		s.Settings.ServerCertPath,
		s.Settings.ServerKeyPath,
	)
	if err != nil {
		return fmt.Errorf("load TLS certs: %v", err)
	}

	host := fmt.Sprintf("%s.%s",
		s.Settings.ServiceName,
		s.Namespace,
	)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		ServerName:   host,
	}

	if len(s.Settings.ClientCAPaths) > 0 {
		roots := x509.NewCertPool()

		for _, caPath := range s.Settings.ClientCAPaths {
			caBytes, err := os.ReadFile(caPath)
			if err != nil {
				return fmt.Errorf("load client CA '%s': %v", caPath, err)
			}

			ok := roots.AppendCertsFromPEM(caBytes)
			if !ok {
				return fmt.Errorf("parse client CA '%s': %v", caPath, err)
			}
		}

		tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConf.ClientCAs = roots
	}

	listenAddr := net.JoinHostPort(s.Settings.ListenAddr, s.Settings.ListenPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("try listen on '%s': %v", listenAddr, err)
	}

	timeout := time.Duration(10) * time.Second

	srv := &http.Server{
		Handler:           s.Router,
		TLSConfig:         tlsConf,
		Addr:              listenAddr,
		IdleTimeout:       timeout,
		ReadTimeout:       timeout,
		ReadHeaderTimeout: timeout,
	}

	doneCh := make(chan struct{})

	s.mu.Lock()
	s.srv = srv
	s.listener = listener
	s.doneCh = doneCh
	s.mu.Unlock()

	go func() {
		defer close(doneCh)
		s.Logger.Info("Webhook server listens", slog.String(pkg.LogKeyAddress, listenAddr))
		if err := srv.ServeTLS(listener, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.Logger.Error("Webhook https server stopped with error", log.Err(err))
		}
	}()

	return nil
}

// Shutdown gracefully stops the webhook https server. It is safe to call when
// the server was never started or has already been shut down — both return nil.
// The provided ctx bounds how long Shutdown waits for in-flight handlers.
func (s *WebhookServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	doneCh := s.doneCh
	s.srv = nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("webhook https server shutdown: %w", err)
	}
	if doneCh != nil {
		select {
		case <-doneCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
