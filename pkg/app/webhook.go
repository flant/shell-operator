package app

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/webhook/defaults"
)

// Note: ValidatingWebhookSettings and ConversionWebhookSettings are defined in
// their respective packages (pkg/webhook/admission and pkg/webhook/conversion).
// Default values are centralized in pkg/webhook/defaults package.

// Validating webhook flag variables - exported so they can be used after parsing
var (
	ValidatingWebhookConfigurationName string
	ValidatingWebhookServiceName       string
	ValidatingWebhookServerCert        string
	ValidatingWebhookServerKey         string
	ValidatingWebhookCA                string
	ValidatingWebhookClientCA          []string
	ValidatingWebhookFailurePolicy     string
	ValidatingWebhookListenPort        string
	ValidatingWebhookListenAddress     string
)

// Conversion webhook flag variables - exported so they can be used after parsing
var (
	ConversionWebhookServiceName   string
	ConversionWebhookServerCert    string
	ConversionWebhookServerKey     string
	ConversionWebhookCA            string
	ConversionWebhookClientCA      []string
	ConversionWebhookListenPort    string
	ConversionWebhookListenAddress string
)

// DefineValidatingWebhookFlags defines flags for ValidatingWebhook server.
func DefineValidatingWebhookFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("validating-webhook-configuration-name", "A name of a ValidatingWebhookConfiguration resource. Can be set with $VALIDATING_WEBHOOK_CONFIGURATION_NAME.").
		Envar("VALIDATING_WEBHOOK_CONFIGURATION_NAME").
		Default(defaults.ValidatingConfigurationName).
		StringVar(&ValidatingWebhookConfigurationName)
	cmd.Flag("validating-webhook-service-name", "A name of a service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVICE_NAME.").
		Envar("VALIDATING_WEBHOOK_SERVICE_NAME").
		Default(defaults.ValidatingServiceName).
		StringVar(&ValidatingWebhookServiceName)
	cmd.Flag("validating-webhook-server-cert", "A path to a server certificate for service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_CERT.").
		Envar("VALIDATING_WEBHOOK_SERVER_CERT").
		Default(defaults.ValidatingServerCertPath).
		StringVar(&ValidatingWebhookServerCert)
	cmd.Flag("validating-webhook-server-key", "A path to a server private key for service used in ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_SERVER_KEY.").
		Envar("VALIDATING_WEBHOOK_SERVER_KEY").
		Default(defaults.ValidatingServerKeyPath).
		StringVar(&ValidatingWebhookServerKey)
	cmd.Flag("validating-webhook-ca", "A path to a ca certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CA.").
		Envar("VALIDATING_WEBHOOK_CA").
		Default(defaults.ValidatingCAPath).
		StringVar(&ValidatingWebhookCA)
	cmd.Flag("validating-webhook-client-ca", "A path to a server certificate for ValidatingWebhookConfiguration. Can be set with $VALIDATING_WEBHOOK_CLIENT_CA.").
		Envar("VALIDATING_WEBHOOK_CLIENT_CA").
		StringsVar(&ValidatingWebhookClientCA)
	cmd.Flag("validating-webhook-failure-policy", "Defines default FailurePolicy for ValidatingWebhookConfiguration.").
		Default(defaults.ValidatingFailurePolicyType).
		Envar("VALIDATING_WEBHOOK_FAILURE_POLICY").
		EnumVar(&ValidatingWebhookFailurePolicy, "Fail", "Ignore")
	cmd.Flag("validating-webhook-listen-port",
		"Defines default ListenPort for ValidatingWebhookConfiguration. "+
			"Can be set with $VALIDATING_WEBHOOK_LISTEN_PORT.").
		Default(defaults.ValidatingListenPort).
		Envar("VALIDATING_WEBHOOK_LISTEN_PORT").
		StringVar(&ValidatingWebhookListenPort)
	cmd.Flag("validating-webhook-listen-address",
		"Defines default ListenAddr for ValidatingWebhookConfiguration. "+
			"Can be set with $VALIDATING_WEBHOOK_LISTEN_ADDRESS.").
		Default(defaults.ValidatingListenAddr).
		Envar("VALIDATING_WEBHOOK_LISTEN_ADDRESS").
		StringVar(&ValidatingWebhookListenAddress)
}

// DefineConversionWebhookFlags defines flags for ConversionWebhook server.
func DefineConversionWebhookFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("conversion-webhook-service-name", "A name of a service for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVICE_NAME.").
		Envar("CONVERSION_WEBHOOK_SERVICE_NAME").
		Default(defaults.ConversionServiceName).
		StringVar(&ConversionWebhookServiceName)
	cmd.Flag("conversion-webhook-server-cert", "A path to a server certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_CERT.").
		Envar("CONVERSION_WEBHOOK_SERVER_CERT").
		Default(defaults.ConversionServerCertPath).
		StringVar(&ConversionWebhookServerCert)
	cmd.Flag("conversion-webhook-server-key", "A path to a server private key for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_SERVER_KEY.").
		Envar("CONVERSION_WEBHOOK_SERVER_KEY").
		Default(defaults.ConversionServerKeyPath).
		StringVar(&ConversionWebhookServerKey)
	cmd.Flag("conversion-webhook-ca", "A path to a ca certificate for clientConfig in CRD. Can be set with $CONVERSION_WEBHOOK_CA.").
		Envar("CONVERSION_WEBHOOK_CA").
		Default(defaults.ConversionCAPath).
		StringVar(&ConversionWebhookCA)
	cmd.Flag("conversion-webhook-client-ca", "A path to a server certificate for CRD.spec.conversion.webhook. Can be set with $CONVERSION_WEBHOOK_CLIENT_CA.").
		Envar("CONVERSION_WEBHOOK_CLIENT_CA").
		StringsVar(&ConversionWebhookClientCA)
	cmd.Flag("conversion-webhook-listen-port",
		"Defines default ListenPort for ConversionWebhookConfiguration. "+
			"Can be set with $CONVERSION_WEBHOOK_LISTEN_PORT.").
		Envar("CONVERSION_WEBHOOK_LISTEN_PORT").
		Default(defaults.ConversionListenPort).
		StringVar(&ConversionWebhookListenPort)
	cmd.Flag("conversion-webhook-listen-address",
		"Defines default ListenAddr for ConversionWebhookConfiguration. "+
			"Can be set with $CONVERSION_WEBHOOK_LISTEN_ADDRESS.").
		Default(defaults.ConversionListenAddr).
		Envar("CONVERSION_WEBHOOK_LISTEN_ADDRESS").
		StringVar(&ConversionWebhookListenAddress)
}
