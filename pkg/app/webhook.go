package app

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

var ValidatingWebhookSettings = &admission.WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: "/validating-certs/tls.crt",
		ServerKeyPath:  "/validating-certs/tls.key",
		ClientCAPaths:  nil,
		ServiceName:    "shell-operator-validating-svc",
		ListenAddr:     "0.0.0.0",
		ListenPort:     "9680",
	},
	CAPath:               "/validating-certs/ca.crt",
	ConfigurationName:    "shell-operator-hooks",
	DefaultFailurePolicy: "Fail",
}

var ConversionWebhookSettings = &conversion.WebhookSettings{
	Settings: server.Settings{
		ServerCertPath: "/conversion-certs/tls.crt",
		ServerKeyPath:  "/conversion-certs/tls.key",
		ClientCAPaths:  nil,
		ServiceName:    "shell-operator-conversion-svc",
		ListenAddr:     "0.0.0.0",
		ListenPort:     "9681",
	},
	CAPath: "/conversion-certs/ca.crt",
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
	cmd.Flag("validating-failure-policy", "Defines default FailurePolicy for ValidatingWebhookConfiguration.").
		Default("Fail").
		Envar("VALIDATING_FAILURE_POLICY").
		EnumVar(&ValidatingWebhookSettings.DefaultFailurePolicy, "Fail", "Ignore")
}

// DefineConversionWebhookFlags defines flags for ConversionWebhook server.
func DefineConversionWebhookFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("conversion-webhook-service-name", "A name of a service for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVICE_NAME.").
		Envar("CONVERSION_WEBHOOK_SERVICE_NAME").
		Default(ConversionWebhookSettings.ServiceName).
		StringVar(&ConversionWebhookSettings.ServiceName)
	cmd.Flag("conversion-webhook-server-cert", "A path to a server certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_CERT.").
		Envar("CONVERSION_WEBHOOK_SERVER_CERT").
		Default(ConversionWebhookSettings.ServerCertPath).
		StringVar(&ConversionWebhookSettings.ServerCertPath)
	cmd.Flag("conversion-webhook-server-key", "A path to a server private key for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_KEY.").
		Envar("CONVERSION_WEBHOOK_SERVER_KEY").
		Default(ConversionWebhookSettings.ServerKeyPath).
		StringVar(&ConversionWebhookSettings.ServerKeyPath)
	cmd.Flag("conversion-webhook-ca", "A path to a ca certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_CA.").
		Envar("CONVERSION_WEBHOOK_CA").
		Default(ConversionWebhookSettings.CAPath).
		StringVar(&ConversionWebhookSettings.CAPath)
	cmd.Flag("conversion-webhook-client-ca", "A path to a server certificate for CRD.spec.conversion.webhook. Can be set with $CONVERSION_WEBHOOK_CLIENT_CA.").
		Envar("CONVERSION_WEBHOOK_CLIENT_CA").
		StringsVar(&ConversionWebhookSettings.ClientCAPaths)
}
