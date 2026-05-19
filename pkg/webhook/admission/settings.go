package admission

import (
	"github.com/flant/shell-operator/pkg/webhook/server"
)

type WebhookSettings struct {
	server.Settings
	CAPath               string
	CABundle             []byte
	ConfigurationName    string
	DefaultFailurePolicy string
}

// DefaultSettings holds the active validating-webhook server settings.
// It is populated by InitFromSettings during operator initialization.
// Tests may use it directly; in production always call InitFromSettings first.
var DefaultSettings = &WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: "/validating-certs/tls.crt",
		ServerKeyPath:  "/validating-certs/tls.key",
		ServiceName:    "shell-operator-validating-svc",
		ListenAddr:     "0.0.0.0",
		ListenPort:     "9680",
	},
	CAPath:               "/validating-certs/ca.crt",
	ConfigurationName:    "shell-operator-hooks",
	DefaultFailurePolicy: "Fail",
}

// InitFromSettings replaces DefaultSettings with values derived from cfg.
// Call this once during operator initialization, after all configuration
// sources (env vars and CLI flags) have been merged into cfg.
func InitFromSettings(s WebhookSettings) {
	DefaultSettings = &s
}
