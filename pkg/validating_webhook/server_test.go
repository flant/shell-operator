package validating_webhook

import (
	"crypto/x509"
	"io/ioutil"
	"testing"

	"github.com/flant/shell-operator/pkg/app"
)

func Test_ServerStart(t *testing.T) {
	app.ValidatingWebhookSettings.ServerKeyPath = "testdata/demo-certs/server-key.pem"
	app.ValidatingWebhookSettings.ServerCertPath = "testdata/demo-certs/server.crt"

	h := NewWebhookHandler()

	srv := &WebhookServer{Router: h.Router}

	err := srv.Start()
	if err != nil {
		t.Fatalf("Server should start: %v", err)
	}
}

func Test_Client_CA(t *testing.T) {
	roots := x509.NewCertPool()

	app.ValidatingWebhookSettings.ClientCAPaths = []string{
		"testdata/demo-certs/client-ca.pem",
	}

	for _, caPath := range app.ValidatingWebhookSettings.ClientCAPaths {
		caBytes, err := ioutil.ReadFile(caPath)
		if err != nil {
			t.Fatalf("ca '%s' should be read: %v", caPath, err)
		}

		ok := roots.AppendCertsFromPEM(caBytes)
		if !ok {
			t.Fatalf("ca '%s' should be parsed", caPath)
		}
	}
}
