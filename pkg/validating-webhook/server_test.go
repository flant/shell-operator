package validating_webhook

import (
	"github.com/flant/shell-operator/pkg/app"
	"testing"
)

func Test_ServerStart(t *testing.T) {
	app.ValidatingWebhookSettings.ServerKeyPath = "testdata/server-key.pem"
	app.ValidatingWebhookSettings.ServerCertPath = "testdata/server.crt"

	h := NewWebhookHandler()

	srv := &WebhookServer{Router: h.Router}

	err := srv.Start()
	if err != nil {
		t.Fatalf("Server should started: %v", err)
	}
}
