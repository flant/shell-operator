package admission

import (
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

type WebhookSettings struct {
	server.Settings
	CAPath               string
	CABundle             []byte
	ConfigurationName    string
	DefaultFailurePolicy string
}

// DefaultSettings returns default settings for validating webhook
// This is initialized at startup and can be modified by flag parsing
var DefaultSettings = &WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: app.ValidatingServerCertPathDefault,
		ServerKeyPath:  app.ValidatingServerKeyPathDefault,
		ClientCAPaths:  nil,
		ServiceName:    app.ValidatingServiceNameDefault,
		ListenAddr:     app.ValidatingListenAddrDefault,
		ListenPort:     app.ValidatingListenPortDefault,
	},
	CAPath:               app.ValidatingCAPathDefault,
	ConfigurationName:    app.ValidatingConfigurationNameDefault,
	DefaultFailurePolicy: app.ValidatingFailurePolicyTypeDefault,
}

// InitFromFlags updates DefaultSettings with values from parsed flags
func InitFromFlags(configName, serviceName, certPath, keyPath, caPath string, clientCAs []string, failurePolicy, port, addr string) {
	if configName != "" {
		DefaultSettings.ConfigurationName = configName
	}
	if serviceName != "" {
		DefaultSettings.ServiceName = serviceName
	}
	if certPath != "" {
		DefaultSettings.ServerCertPath = certPath
	}
	if keyPath != "" {
		DefaultSettings.ServerKeyPath = keyPath
	}
	if caPath != "" {
		DefaultSettings.CAPath = caPath
	}
	if len(clientCAs) > 0 {
		DefaultSettings.ClientCAPaths = clientCAs
	}
	if failurePolicy != "" {
		DefaultSettings.DefaultFailurePolicy = failurePolicy
	}
	if port != "" {
		DefaultSettings.ListenPort = port
	}
	if addr != "" {
		DefaultSettings.ListenAddr = addr
	}
}
