// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shell_operator

import (
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/app"
)

// ShellOperatorConfig holds configuration for ShellOperator initialization
type ShellOperatorConfig struct {
	Logger              *log.Logger
	ListenAddress       string
	ListenPort          string
	HooksDir            string
	TempDir             string
	DebugUnixSocket     string
	DebugHttpServerAddr string
	MetricStorage       metricsstorage.Storage
	HookMetricStorage   metricsstorage.Storage
}

// ConfigOption defines a functional option for ShellOperatorConfig
type ConfigOption func(*ShellOperatorConfig)

// NewShellOperatorConfig creates a new configuration with default values and applies the provided options.
// If no logger is provided via options, a default logger will be created.
func NewShellOperatorConfig(options ...ConfigOption) *ShellOperatorConfig {
	config := &ShellOperatorConfig{
		ListenAddress:       app.ListenAddress,
		ListenPort:          app.ListenPort,
		HooksDir:            app.HooksDir,
		TempDir:             app.TempDir,
		DebugUnixSocket:     app.DebugUnixSocket,
		DebugHttpServerAddr: app.DebugHttpServerAddr,
	}

	// Apply all provided options
	for _, option := range options {
		option(config)
	}

	// Set default logger if none provided
	if config.Logger == nil {
		config.Logger = log.NewLogger().Named("shell-operator")
	}

	// Use provided metric storage or create default
	if config.MetricStorage == nil {
		config.MetricStorage = metricsstorage.NewMetricStorage(
			metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
			metricsstorage.WithLogger(config.Logger.Named("metric-storage")),
		)
	}

	// Use provided hook metric storage or create default
	if config.HookMetricStorage == nil {
		config.HookMetricStorage = metricsstorage.NewMetricStorage(
			metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
			metricsstorage.WithNewRegistry(),
			metricsstorage.WithLogger(config.Logger.Named("hook-metric-storage")),
		)
	}

	return config
}

// Validate validates the configuration and returns an error if invalid
func (cfg *ShellOperatorConfig) Validate() error {
	if cfg.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	if cfg.ListenAddress == "" {
		return fmt.Errorf("listen address cannot be empty")
	}
	if cfg.ListenPort == "" {
		return fmt.Errorf("listen port cannot be empty")
	}
	if cfg.HooksDir == "" {
		return fmt.Errorf("hooks directory cannot be empty")
	}
	if cfg.TempDir == "" {
		return fmt.Errorf("temp directory cannot be empty")
	}
	return nil
}

// WithLogger sets the logger for the operator configuration
func WithLogger(logger *log.Logger) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.Logger = logger
	}
}

// WithListenAddress sets the listen address for the HTTP server
func WithListenAddress(address string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.ListenAddress = address
	}
}

// WithListenPort sets the listen port for the HTTP server
func WithListenPort(port string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.ListenPort = port
	}
}

// WithHooksDir sets the directory containing hooks
func WithHooksDir(dir string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.HooksDir = dir
	}
}

// WithTempDir sets the temporary directory
func WithTempDir(dir string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.TempDir = dir
	}
}

// WithDebugUnixSocket sets the debug unix socket path
func WithDebugUnixSocket(socket string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.DebugUnixSocket = socket
	}
}

// WithDebugHttpServerAddr sets the debug HTTP server address
func WithDebugHttpServerAddr(addr string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.DebugHttpServerAddr = addr
	}
}

// WithMetricStorage sets the metric storage for built-in metrics
func WithMetricStorage(storage metricsstorage.Storage) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.MetricStorage = storage
	}
}

// WithHookMetricStorage sets the metric storage for hook metrics
func WithHookMetricStorage(storage metricsstorage.Storage) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.HookMetricStorage = storage
	}
}

// Convenience options that combine multiple settings

// WithListenConfig sets both listen address and port
func WithListenConfig(address, port string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.ListenAddress = address
		config.ListenPort = port
	}
}

// WithDirectories sets both hooks and temp directories
func WithDirectories(hooksDir, tempDir string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.HooksDir = hooksDir
		config.TempDir = tempDir
	}
}

// WithDebugConfig sets both debug unix socket and HTTP server address
func WithDebugConfig(unixSocket, httpAddr string) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.DebugUnixSocket = unixSocket
		config.DebugHttpServerAddr = httpAddr
	}
}

// WithMetricStorages sets both metric storage and hook metric storage
func WithMetricStorages(metricStorage, hookMetricStorage metricsstorage.Storage) ConfigOption {
	return func(config *ShellOperatorConfig) {
		config.MetricStorage = metricStorage
		config.HookMetricStorage = hookMetricStorage
	}
}
