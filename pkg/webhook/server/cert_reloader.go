package server

import (
	"crypto/tls"
	"os"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// certReloader serves a TLS key pair and reloads it when the cert or key file
// changes on disk, so a renewed certificate is used without a restart.
type certReloader struct {
	certPath, keyPath string
	logger            *log.Logger

	mu              sync.Mutex
	cert            *tls.Certificate
	certMod, keyMod time.Time
}

func newCertReloader(certPath, keyPath string, logger *log.Logger) (*certReloader, error) {
	r := &certReloader{certPath: certPath, keyPath: keyPath, logger: logger}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

// load reads the key pair and records the mtimes it was read at. Callers hold mu or own r exclusively.
func (r *certReloader) load() error {
	certInfo, err := os.Stat(r.certPath)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(r.keyPath)
	if err != nil {
		return err
	}
	pair, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return err
	}
	r.cert, r.certMod, r.keyMod = &pair, certInfo.ModTime(), keyInfo.ModTime()
	return nil
}

// getCertificate is a tls.Config.GetCertificate callback. If the files changed
// but cannot be loaded (for example a half-written renewal), it keeps serving
// the previous pair and retries on the next handshake.
func (r *certReloader) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	certInfo, certErr := os.Stat(r.certPath)
	keyInfo, keyErr := os.Stat(r.keyPath)
	if certErr == nil && keyErr == nil &&
		(!certInfo.ModTime().Equal(r.certMod) || !keyInfo.ModTime().Equal(r.keyMod)) {
		if err := r.load(); err != nil {
			r.logger.Warn("Reload of renewed TLS certificate failed, keep serving the previous one", log.Err(err))
		} else {
			r.logger.Info("TLS certificate reloaded")
		}
	}
	return r.cert, nil
}
