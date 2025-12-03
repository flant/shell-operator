package conversion

import (
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

type WebhookSettings struct {
	server.Settings
	CAPath   string
	CABundle []byte
}

// DefaultSettings returns default settings for conversion webhook
// This is initialized at startup and can be modified by flag parsing
var DefaultSettings = &WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: app.ConversionServerCertPathDefault,
		ServerKeyPath:  app.ConversionServerKeyPathDefault,
		ClientCAPaths:  nil,
		ServiceName:    app.ConversionServiceNameDefault,
		ListenAddr:     app.ConversionListenAddrDefault,
		ListenPort:     app.ConversionListenPortDefault,
	},
	CAPath: app.ConversionCAPathDefault,
}

// InitFromFlags updates DefaultSettings with values from parsed flags
func InitFromFlags(serviceName, certPath, keyPath, caPath string, clientCAs []string, port, addr string) {
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
	if port != "" {
		DefaultSettings.ListenPort = port
	}
	if addr != "" {
		DefaultSettings.ListenAddr = addr
	}
}
