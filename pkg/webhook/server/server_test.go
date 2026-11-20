package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/go-chi/chi/v5"
)

func Test_ServerStart(t *testing.T) {
	s := &Settings{
		ServerCertPath: "testdata/demo-certs/server.crt",
		ServerKeyPath:  "testdata/demo-certs/server-key.pem",
	}

	rtr := chi.NewRouter()

	srv := NewWebhookServer(s, "", rtr, log.NewNop())

	err := srv.Start()
	if err != nil {
		t.Fatalf("Server should start: %v", err)
	}
}

// writeSelfSignedPair writes a fresh cert/key pair with the given serial and
// sets both files' mtime to modTime so tests do not depend on clock granularity.
func writeSelfSignedPair(t *testing.T, certPath, keyPath string, serial int64, modTime time.Time) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	for path, data := range map[string][]byte{certPath: certPEM, keyPath: keyPEM} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
}

func servedSerial(t *testing.T, addr string) int64 {
	t.Helper()

	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test: only the leaf is inspected
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates[0].SerialNumber.Int64()
}

func Test_ServerReloadsRenewedCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	now := time.Now()
	writeSelfSignedPair(t, certPath, keyPath, 1, now)

	srv := NewWebhookServer(&Settings{
		ServerCertPath: certPath,
		ServerKeyPath:  keyPath,
		ListenAddr:     "127.0.0.1",
		ListenPort:     "0",
	}, "", chi.NewRouter(), log.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatalf("Server should start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	addr := srv.listener.Addr().String()
	if got := servedSerial(t, addr); got != 1 {
		t.Fatalf("before renewal: serial = %d, want 1", got)
	}

	// A certificate manager renews the pair in place.
	writeSelfSignedPair(t, certPath, keyPath, 2, now.Add(time.Minute))

	if got := servedSerial(t, addr); got != 2 {
		t.Fatalf("after renewal: serial = %d, want 2", got)
	}
}

func Test_Client_CA(t *testing.T) {
	roots := x509.NewCertPool()

	s := Settings{}
	s.ClientCAPaths = []string{
		"testdata/demo-certs/client-ca.pem",
	}

	for _, caPath := range s.ClientCAPaths {
		caBytes, err := os.ReadFile(caPath)
		if err != nil {
			t.Fatalf("ca '%s' should be read: %v", caPath, err)
		}

		ok := roots.AppendCertsFromPEM(caBytes)
		if !ok {
			t.Fatalf("ca '%s' should be parsed", caPath)
		}
	}
}
