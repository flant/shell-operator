package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
)

type WebhookServer struct {
	Settings  *Settings
	Namespace string
	Router    chi.Router
}

// Start runs https server to listen for AdmissionReview requests from the API-server.
func (s *WebhookServer) Start() error {
	// Load server certificate.
	keyPair, err := tls.LoadX509KeyPair(
		s.Settings.ServerCertPath,
		s.Settings.ServerKeyPath,
	)
	if err != nil {
		return fmt.Errorf("load TLS certs: %v", err)
	}

	// Construct a hostname for certificate.
	host := fmt.Sprintf("%s.%s",
		s.Settings.ServiceName,
		s.Namespace,
	)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		ServerName:   host,
	}

	// Load client CA if defined
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
	// Check if port is available
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

	go func() {
		log.Info("Webhook server listens", slog.String("address", listenAddr))
		err := srv.ServeTLS(listener, "", "")
		if err != nil {
			log.Error("Error starting Webhook https server", log.Err(err))
			// Stop process if server can't start.
			os.Exit(1)
		}
	}()

	return nil
}
