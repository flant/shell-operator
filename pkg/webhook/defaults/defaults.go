package defaults

// Default values for validating webhook configuration
const (
	ValidatingServerCertPath    = "/validating-certs/tls.crt"
	ValidatingServerKeyPath     = "/validating-certs/tls.key"
	ValidatingServiceName       = "shell-operator-validating-svc"
	ValidatingListenAddr        = "0.0.0.0"
	ValidatingListenPort        = "9680"
	ValidatingCAPath            = "/validating-certs/ca.crt"
	ValidatingConfigurationName = "shell-operator-hooks"
	ValidatingFailurePolicyType = "Fail"
)

// Default values for conversion webhook configuration
const (
	ConversionServerCertPath = "/conversion-certs/tls.crt"
	ConversionServerKeyPath  = "/conversion-certs/tls.key"
	ConversionServiceName    = "shell-operator-conversion-svc"
	ConversionListenAddr     = "0.0.0.0"
	ConversionListenPort     = "9681"
	ConversionCAPath         = "/conversion-certs/ca.crt"
)
