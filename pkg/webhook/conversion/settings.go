package conversion

import (
	"github.com/flant/shell-operator/pkg/webhook/server"
)

// WebhookSettings holds the configuration for a conversion-webhook server.
// Library consumers build a *WebhookSettings explicitly and assign it to the
// WebhookManager's Settings field — there is no package-level singleton.
type WebhookSettings struct {
	server.Settings
	CAPath   string
	CABundle []byte
}
