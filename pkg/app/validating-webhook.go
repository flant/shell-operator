package app

import (
	"gopkg.in/alecthomas/kingpin.v2"
)

type validatingWebhookSettings struct {
	ServerCertPath    string
	ServerKeyPath     string
	CAPath            string
	CABundle          []byte
	ClientCAPaths     []string
	ServiceName       string
	ConfigurationName string
	ListenPort        string
	ListenAddr        string
}

var ValidatingWebhookSettings = &validatingWebhookSettings{
	ServerCertPath:    "/validating-certs/server.crt",
	ServerKeyPath:     "/validating-certs/server-key.pem",
	CAPath:            "/validating-certs/ca.pem",
	ClientCAPaths:     nil,
	ServiceName:       "shell-operator-validating-svc",
	ConfigurationName: "shell-operator-hooks",
	ListenAddr:        "0.0.0.0",
	ListenPort:        "9680",
}

// DefineValidatingWebhookFlags defines flags for ValidatingWebhook server.
func DefineValidatingWebhookFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("validating-webhook-configuration-name", "A name of a ValidatingWebhookConfiguration resource. Can be set with $VALIDATING_WEBHOOK_CONFIGURATION_NAME.").
		Envar("VALIDATING_WEBHOOK_CONFIGURATION_NAME").
		Default(ValidatingWebhookSettings.ConfigurationName).
		StringVar(&ValidatingWebhookSettings.ConfigurationName)
	cmd.Flag("validating-webhook-service-name", "A name of a service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.").
		Envar("VALIDATING_WEBHOOK_SERVICE_NAME").
		Default(ValidatingWebhookSettings.ServiceName).
		StringVar(&ValidatingWebhookSettings.ServiceName)
	cmd.Flag("validating-webhook-server-cert", "A path to a server certificate for service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.").
		Envar("VALIDATING_WEBHOOK_SERVER_CERT").
		Default(ValidatingWebhookSettings.ServerCertPath).
		StringVar(&ValidatingWebhookSettings.ServerCertPath)
	cmd.Flag("validating-webhook-server-key", "A path to a server private key for service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.").
		Envar("VALIDATING_WEBHOOK_SERVER_KEY").
		Default(ValidatingWebhookSettings.ServerKeyPath).
		StringVar(&ValidatingWebhookSettings.ServerKeyPath)
	cmd.Flag("validating-webhook-ca", "A path to a ca certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CA.").
		Envar("VALIDATING_WEBHOOK_CA").
		Default(ValidatingWebhookSettings.CAPath).
		StringVar(&ValidatingWebhookSettings.CAPath)
	cmd.Flag("validating-webhook-client-ca", "A path to a server certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.").
		Envar("VALIDATING_WEBHOOK_CLIENT_CA").
		StringsVar(&ValidatingWebhookSettings.ClientCAPaths)
}
