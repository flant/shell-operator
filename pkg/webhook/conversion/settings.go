package conversion

import (
"github.com/flant/shell-operator/pkg/webhook/server"
)

type WebhookSettings struct {
	server.Settings
	CAPath   string
	CABundle []byte
}

// DefaultSettings holds the active conversion-webhook server settings.
// It is populated by InitFromSettings during operator initialization.
// Tests may use it directly; in production always call InitFromSettings first.
var DefaultSettings = &WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: "/conversion-certs/tls.crt",
		ServerKeyPath:  "/conversion-certs/tls.key",
		ServiceName:    "shell-operator-conversion-svc",
		ListenAddr:     "0.0.0.0",
		ListenPort:     "9681",
	},
	CAPath: "/conversion-certs/ca.crt",
}

// InitFromSettings replaces DefaultSettings with values derived from cfg.
// Call this once during operator initialization, after all configuration
// sources (env vars and CLI flags) have been merged into cfg.
func InitFromSettings(s WebhookSettings) {
	DefaultSettings = &s
}
