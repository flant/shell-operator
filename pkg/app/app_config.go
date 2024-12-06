package app

import (
	"fmt"
	"time"

	env "github.com/caarlos0/env/v11"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type appConfig struct {
	HooksDir                string `env:"HOOKS_DIR"`
	TmpDir                  string `env:"TMP_DIR"`
	ListenAddress           string `env:"LISTEN_ADDRESS"`
	ListenPort              string `env:"LISTEN_PORT"`
	PrometheusMetricsPrefix string `env:"PROMETHEUS_METRICS_PREFIX"`
	// unused?
	HooksMetricsListenPort string `env:"HOOK_METRICS_LISTEN_PORT"`
	Namespace              string `env:"NAMESPACE"`
}

func newAppConfig() *appConfig {
	return &appConfig{}
}

type debugConfig struct {
	HTTPServerAddress  string `env:"HTTP_SERVER_ADDR"`
	KeepTemporaryFiles string `env:"KEEP_TMP_FILES"`
	KubernetesAPI      bool   `env:"KUBERNETES_API"`
	UnixSocket         string `env:"UNIX_SOCKET"`
}

func newDebugConfig() *debugConfig {
	return &debugConfig{}
}

type jqConfig struct {
	LibraryPath string `env:"LIBRARY_PATH"`
}

func newJQConfig() *jqConfig {
	return &jqConfig{}
}

type kubeConfig struct {
	// Settings for Kubernetes connection.
	ContextName   string `env:"CONTEXT"`
	ConfigPath    string `env:"CONFIG"`
	ServerAddress string `env:"SERVER"`
	// Rate limit settings for 'main' kube client
	ClientQPS   float32 `env:"CLIENT_QPS"`
	ClientBurst int     `env:"CLIENT_BURST"`
}

func newKubeConfig() *kubeConfig {
	return &kubeConfig{}
}

type objectPatcherConfig struct {
	// Settings for 'object_patcher' kube client
	KubeClientQPS     float32       `env:"KUBE_CLIENT_QPS"`
	KubeClisntBurst   int           `env:"KUBE_CLIENT_BURST"`
	KubeClientTimeout time.Duration `env:"KUBE_CLIENT_TIMEOUT"`
}

func newObjectPatcherConfig() *objectPatcherConfig {
	return &objectPatcherConfig{}
}

type validatingWebhookConfig struct {
	ConfigurationName string `env:"CONFIGURATION_NAME"`
	ServiceName       string `env:"SERVICE_NAME"`
	ServerCert        string `env:"SERVER_CERT"`
	ServerKey         string `env:"SERVER_KEY"`
	CA                string `env:"CA"`
	// check separator?
	ClientCA []string `env:"CLIENT_CA" envSeparator:","`
	// enum "Fail" || "Ignore"
	FailurePolicy string `env:"FAILURE_POLICY"`
	ListenPort    string `env:"LISTEN_PORT"`
	ListenAddress string `env:"LISTEN_ADDRESS"`
}

func newValidatingWebhookConfig() *validatingWebhookConfig {
	return &validatingWebhookConfig{}
}

type conversionWebhookConfig struct {
	ServiceName string `env:"SERVICE_NAME"`
	ServerCert  string `env:"SERVER_CERT"`
	ServerKey   string `env:"SERVER_KEY"`
	CA          string `env:"CA"`
	// check separator?
	ClientCA      []string `env:"CLIENT_CA" envSeparator:","`
	ListenPort    string   `env:"LISTEN_PORT"`
	ListenAddress string   `env:"LISTEN_ADDRESS"`
}

func newConversionWebhookConfig() *conversionWebhookConfig {
	return &conversionWebhookConfig{}
}

type logConfig struct {
	Level         string `env:"LEVEL"`
	Type          string `env:"TYPE"`
	NoTime        bool   `env:"NO_TIME"`
	ProxyHookJson bool   `env:"PROXY_HOOK_JSON"`
}

func newLogConfig() *logConfig {
	return &logConfig{}
}

type Config struct {
	AppConfig               *appConfig               `envPrefix:"SHELL_OPERATOR_"`
	JQConfig                *jqConfig                `envPrefix:"JQ_"`
	KubeConfig              *kubeConfig              `envPrefix:"KUBE_"`
	ObjectPatcherConfig     *objectPatcherConfig     `envPrefix:"OBJECT_PATCHER_"`
	ValidatingWebhookConfig *validatingWebhookConfig `envPrefix:"VALIDATING_WEBHOOK_"`
	ConversionWebhookConfig *conversionWebhookConfig `envPrefix:"CONVERSION_WEBHOOK_"`

	DebugConfig *debugConfig `envPrefix:"DEBUG_"`

	LogConfig *logConfig `envPrefix:"LOG_"`
	LogLevel  log.Level  `env:"-"`

	ready bool
}

func NewConfig() *Config {
	return &Config{
		AppConfig:               newAppConfig(),
		JQConfig:                newJQConfig(),
		KubeConfig:              newKubeConfig(),
		ObjectPatcherConfig:     newObjectPatcherConfig(),
		ValidatingWebhookConfig: newValidatingWebhookConfig(),
		ConversionWebhookConfig: newConversionWebhookConfig(),
		DebugConfig:             newDebugConfig(),
		LogConfig:               newLogConfig(),
	}
}

func (cfg *Config) Parse() error {
	if cfg.IsReady() {
		return nil
	}

	opts := env.Options{
		Prefix: "",
	}

	err := env.ParseWithOptions(cfg, opts)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.LogLevel = log.LogLevelFromStr(cfg.LogConfig.Level)

	return nil
}

func (cfg *Config) SetupGlobalVars() {
	if cfg.IsReady() {
		return
	}

	setIfNotEmpty(&HooksDir, cfg.AppConfig.HooksDir)
	setIfNotEmpty(&TempDir, cfg.AppConfig.TmpDir)
	setIfNotEmpty(&ListenAddress, cfg.AppConfig.ListenAddress)
	setIfNotEmpty(&ListenPort, cfg.AppConfig.ListenPort)
	setIfNotEmpty(&PrometheusMetricsPrefix, cfg.AppConfig.PrometheusMetricsPrefix)
	setIfNotEmpty(&Namespace, cfg.AppConfig.Namespace)

	setIfNotEmpty(&DebugHttpServerAddr, cfg.DebugConfig.HTTPServerAddress)
	setIfNotEmpty(&DebugKeepTmpFilesVar, cfg.DebugConfig.KeepTemporaryFiles)
	setIfNotEmpty(&DebugKeepTmpFiles, cfg.DebugConfig.KeepTemporaryFiles == "true" || cfg.DebugConfig.KeepTemporaryFiles == "yes")
	setIfNotEmpty(&DebugKubernetesAPI, cfg.DebugConfig.KubernetesAPI)
	setIfNotEmpty(&DebugUnixSocket, cfg.DebugConfig.UnixSocket)

	setIfNotEmpty(&JqLibraryPath, cfg.JQConfig.LibraryPath)

	setIfNotEmpty(&KubeContext, cfg.KubeConfig.ContextName)
	setIfNotEmpty(&KubeConfig, cfg.KubeConfig.ConfigPath)
	setIfNotEmpty(&KubeServer, cfg.KubeConfig.ServerAddress)
	setIfNotEmpty(&KubeClientQps, cfg.KubeConfig.ClientQPS)
	setIfNotEmpty(&KubeClientBurst, cfg.KubeConfig.ClientBurst)

	setIfNotEmpty(&ObjectPatcherKubeClientQps, cfg.ObjectPatcherConfig.KubeClientQPS)
	setIfNotEmpty(&ObjectPatcherKubeClientBurst, cfg.ObjectPatcherConfig.KubeClisntBurst)
	setIfNotEmpty(&ObjectPatcherKubeClientTimeout, cfg.ObjectPatcherConfig.KubeClientTimeout)

	setIfNotEmpty(&LogLevel, cfg.LogConfig.Level)
	setIfNotEmpty(&LogNoTime, cfg.LogConfig.NoTime)
	setIfNotEmpty(&LogType, cfg.LogConfig.Type)
	setIfNotEmpty(&LogProxyHookJSON, cfg.LogConfig.ProxyHookJson)

	setIfNotEmpty(&ValidatingWebhookSettings.ConfigurationName, cfg.ValidatingWebhookConfig.ConfigurationName)
	setIfNotEmpty(&ValidatingWebhookSettings.ServiceName, cfg.ValidatingWebhookConfig.ServiceName)
	setIfNotEmpty(&ValidatingWebhookSettings.ServerCertPath, cfg.ValidatingWebhookConfig.ServerCert)
	setIfNotEmpty(&ValidatingWebhookSettings.ServerKeyPath, cfg.ValidatingWebhookConfig.ServerKey)
	setIfNotEmpty(&ValidatingWebhookSettings.CAPath, cfg.ValidatingWebhookConfig.CA)
	setSliceIfNotEmpty(&ValidatingWebhookSettings.ClientCAPaths, cfg.ValidatingWebhookConfig.ClientCA)
	setIfNotEmpty(&ValidatingWebhookSettings.DefaultFailurePolicy, cfg.ValidatingWebhookConfig.FailurePolicy)
	setIfNotEmpty(&ValidatingWebhookSettings.ListenPort, cfg.ValidatingWebhookConfig.ListenPort)
	setIfNotEmpty(&ValidatingWebhookSettings.ListenAddr, cfg.ValidatingWebhookConfig.ListenAddress)

	setIfNotEmpty(&ConversionWebhookSettings.ServiceName, cfg.ValidatingWebhookConfig.ServiceName)
	setIfNotEmpty(&ConversionWebhookSettings.ServerCertPath, cfg.ValidatingWebhookConfig.ServerCert)
	setIfNotEmpty(&ConversionWebhookSettings.ServerKeyPath, cfg.ValidatingWebhookConfig.ServerKey)
	setIfNotEmpty(&ConversionWebhookSettings.CAPath, cfg.ValidatingWebhookConfig.CA)
	setSliceIfNotEmpty(&ConversionWebhookSettings.ClientCAPaths, cfg.ValidatingWebhookConfig.ClientCA)
	setIfNotEmpty(&ConversionWebhookSettings.ListenPort, cfg.ValidatingWebhookConfig.ListenPort)
	setIfNotEmpty(&ConversionWebhookSettings.ListenAddr, cfg.ValidatingWebhookConfig.ListenAddress)
}

func (cfg *Config) IsReady() bool {
	return cfg.ready
}

func (cfg *Config) SetReady() {
	cfg.ready = true
}

var configInstance *Config

func MustGetConfig() *Config {
	cfg, err := GetConfig()
	if err != nil {
		panic(err)
	}

	return cfg
}

func GetConfig() (*Config, error) {
	if configInstance != nil {
		return configInstance, nil
	}

	cfg := NewConfig()
	err := cfg.Parse()
	if err != nil {
		return nil, err
	}

	configInstance = cfg

	return configInstance, nil
}

func setIfNotEmpty[T comparable](v *T, env T) {
	if !isZero(env) {
		*v = env
	}
}

func setSliceIfNotEmpty[T any](v *[]T, env []T) {
	if len(env) != 0 {
		*v = env
	}
}

func isZero[T comparable](v T) bool {
	return v == *new(T)
}
