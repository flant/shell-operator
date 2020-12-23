package validating_webhook

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
)

type WebhookServer struct {
	Router chi.Router
}

// StartWebhookServer starts https server
// to listen for AdmissionReview requests from cluster
func (s *WebhookServer) Start() error {
	// Load server certificate
	keyPair, err := tls.LoadX509KeyPair(
		app.ValidatingWebhookSettings.ServerCertPath,
		app.ValidatingWebhookSettings.ServerKeyPath,
	)
	if err != nil {
		return fmt.Errorf("load TLS certs: %v", err)
	}

	// Construct hostname
	host := fmt.Sprintf("%s.%s",
		app.ValidatingWebhookSettings.ServiceName,
		app.Namespace,
	)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
		ServerName:   host,
	}

	// Load client CA if defined
	if len(app.ValidatingWebhookSettings.ClientCAPaths) > 0 {
		roots := x509.NewCertPool()

		for _, caPath := range app.ValidatingWebhookSettings.ClientCAPaths {
			caBytes, err := ioutil.ReadFile(caPath)
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

	listenAddr := app.ValidatingWebhookSettings.ListenAddr + ":" + app.ValidatingWebhookSettings.ListenPort
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
		log.Infof("Webhook server listens on %s", listenAddr)
		err := srv.ServeTLS(listener, "", "")
		if err != nil {
			log.Errorf("Error starting Webhook https server: %v", err)
			//os.Exit(1)
		}
	}()

	return nil
}
