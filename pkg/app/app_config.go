package app

import (
	"fmt"
	"time"

	env "github.com/caarlos0/env/v11"
)

// AppSettings holds shell-operator's primary runtime settings.
// Defaults are set in NewConfig; env tags declare the variable name only.
type AppSettings struct {
	HooksDir                string `env:"HOOKS_DIR"`
	TempDir                 string `env:"TMP_DIR"`
	ListenAddress           string `env:"LISTEN_ADDRESS"`
	ListenPort              string `env:"LISTEN_PORT"`
	PrometheusMetricsPrefix string `env:"PROMETHEUS_METRICS_PREFIX"`
	Namespace               string `env:"NAMESPACE"`
}

// KubeSettings holds Kubernetes connection parameters.
type KubeSettings struct {
	Context     string  `env:"CONTEXT"`
	Config      string  `env:"CONFIG"`
	Server      string  `env:"SERVER"`
	ClientQPS   float32 `env:"CLIENT_QPS"`
	ClientBurst int     `env:"CLIENT_BURST"`
}

// ObjectPatcherSettings holds settings for the object-patcher Kubernetes client.
type ObjectPatcherSettings struct {
	KubeClientQPS     float32       `env:"KUBE_CLIENT_QPS"`
	KubeClientBurst   int           `env:"KUBE_CLIENT_BURST"`
	KubeClientTimeout time.Duration `env:"KUBE_CLIENT_TIMEOUT"`
}

// AdmissionSettings holds settings for the validating-webhook server.
type AdmissionSettings struct {
	ConfigurationName string   `env:"CONFIGURATION_NAME"`
	ServiceName       string   `env:"SERVICE_NAME"`
	ServerCert        string   `env:"SERVER_CERT"`
	ServerKey         string   `env:"SERVER_KEY"`
	CA                string   `env:"CA"`
	ClientCA          []string `env:"CLIENT_CA" envSeparator:","`
	FailurePolicy     string   `env:"FAILURE_POLICY"`
	ListenPort        string   `env:"LISTEN_PORT"`
	ListenAddress     string   `env:"LISTEN_ADDRESS"`
}

// ConversionSettings holds settings for the conversion-webhook server.
type ConversionSettings struct {
	ServiceName   string   `env:"SERVICE_NAME"`
	ServerCert    string   `env:"SERVER_CERT"`
	ServerKey     string   `env:"SERVER_KEY"`
	CA            string   `env:"CA"`
	ClientCA      []string `env:"CLIENT_CA" envSeparator:","`
	ListenPort    string   `env:"LISTEN_PORT"`
	ListenAddress string   `env:"LISTEN_ADDRESS"`
}

// DebugSettings holds settings for the debug server.
type DebugSettings struct {
	UnixSocket     string `env:"UNIX_SOCKET"`
	HTTPServerAddr string `env:"HTTP_SERVER_ADDR"`
	KeepTempFiles  bool   `env:"KEEP_TMP_FILES"`
	KubernetesAPI  bool   `env:"KUBERNETES_API"`
}

// LogSettings holds logging configuration.
type LogSettings struct {
	Level         string `env:"LEVEL"`
	Type          string `env:"TYPE"`
	NoTime        bool   `env:"NO_TIME"`
	ProxyHookJSON bool   `env:"PROXY_HOOK_JSON"`
}

// Config is the single source of truth for operator configuration.
// Populate it in stages: NewConfig sets hardcoded defaults,
// ParseEnv overrides with environment variables, BindFlags (in pkg/app/flags.go)
// registers CLI flags whose defaults are the current cfg values so that an
// explicit flag always wins. Priority: CLI flags > env vars > hardcoded defaults.
type Config struct {
	App           AppSettings           `envPrefix:"SHELL_OPERATOR_"`
	Kube          KubeSettings          `envPrefix:"KUBE_"`
	ObjectPatcher ObjectPatcherSettings `envPrefix:"OBJECT_PATCHER_"`
	Admission     AdmissionSettings     `envPrefix:"VALIDATING_WEBHOOK_"`
	Conversion    ConversionSettings    `envPrefix:"CONVERSION_WEBHOOK_"`
	Debug         DebugSettings         `envPrefix:"DEBUG_"`
	Log           LogSettings           `envPrefix:"LOG_"`
}

// NewConfig returns a Config populated with all hardcoded defaults.
func NewConfig() *Config {
	return &Config{
		App: AppSettings{
			HooksDir:                "hooks",
			TempDir:                 "/tmp/shell-operator",
			ListenAddress:           "0.0.0.0",
			ListenPort:              "9115",
			PrometheusMetricsPrefix: "shell_operator_",
		},
		Kube: KubeSettings{
			ClientQPS:   5,
			ClientBurst: 10,
		},
		ObjectPatcher: ObjectPatcherSettings{
			KubeClientQPS:     5,
			KubeClientBurst:   10,
			KubeClientTimeout: 10 * time.Second,
		},
		Admission: AdmissionSettings{
			ConfigurationName: "shell-operator-hooks",
			ServiceName:       "shell-operator-validating-svc",
			ServerCert:        "/validating-certs/tls.crt",
			ServerKey:         "/validating-certs/tls.key",
			CA:                "/validating-certs/ca.crt",
			FailurePolicy:     "Fail",
			ListenPort:        "9680",
			ListenAddress:     "0.0.0.0",
		},
		Conversion: ConversionSettings{
			ServiceName:   "shell-operator-conversion-svc",
			ServerCert:    "/conversion-certs/tls.crt",
			ServerKey:     "/conversion-certs/tls.key",
			CA:            "/conversion-certs/ca.crt",
			ListenPort:    "9681",
			ListenAddress: "0.0.0.0",
		},
		Debug: DebugSettings{
			UnixSocket: "/var/run/shell-operator/debug.socket",
		},
		Log: LogSettings{
			Level: "info",
			Type:  "text",
		},
	}
}

// ParseEnv overrides cfg fields with values from environment variables.
// Fields whose env var is not set retain their current values, so hardcoded
// defaults from NewConfig are preserved when no env var is present.
// Call after NewConfig so the hardcoded defaults serve as the baseline.
func ParseEnv(cfg *Config) error {
	if err := env.ParseWithOptions(cfg, env.Options{}); err != nil {
		return fmt.Errorf("parse config from environment: %w", err)
	}
	return nil
}

// Validate returns an error if cfg contains invalid or inconsistent values.
func Validate(cfg *Config) error {
	if cfg.App.HooksDir == "" {
		return fmt.Errorf("hooks directory must not be empty (set --hooks-dir or $SHELL_OPERATOR_HOOKS_DIR)")
	}
	if cfg.App.TempDir == "" {
		return fmt.Errorf("temp directory must not be empty (set --tmp-dir or $SHELL_OPERATOR_TMP_DIR)")
	}
	if cfg.App.ListenPort == "" {
		return fmt.Errorf("listen port must not be empty (set --listen-port or $SHELL_OPERATOR_LISTEN_PORT)")
	}
	if cfg.Admission.FailurePolicy != "Fail" && cfg.Admission.FailurePolicy != "Ignore" {
		return fmt.Errorf("validating webhook failure policy must be 'Fail' or 'Ignore', got %q", cfg.Admission.FailurePolicy)
	}
	return nil
}
