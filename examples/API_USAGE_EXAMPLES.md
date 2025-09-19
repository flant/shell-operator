# Shell Operator API Usage Examples

## Complete API Reference

The Shell Operator now provides a flexible, options-based API for initialization. Here are all the available ways to create a ShellOperator instance:

### 1. Basic Constructor (Original)
```go
import (
    "github.com/flant/shell-operator/pkg/shell-operator"
    "log"
)

// Simple constructor - uses default logger and no metrics storage
operator := shell_operator.NewShellOperator()
```

### 2. Configuration-Based Constructor
```go
import (
    "context"
    "github.com/flant/shell-operator/pkg/shell-operator"
    "github.com/flant/shell-operator/pkg/shell-operator/config"
)

// Using the flexible configuration system
operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    config.WithLogger(logger),
    config.WithMetricStorage(storage),
    config.WithHookMetricStorage(hookStorage), 
    config.WithHooksDir("/custom/hooks"),
)
```

### 3. With Separate Metric Storages
```go
import (
    "github.com/flant/shell-operator/pkg/shell-operator"
    metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

builtinStorage := metricsstorage.NewStorage("builtin-metrics")
hookStorage := metricsstorage.NewStorage("hook-metrics")

operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    shell_operator.WithMetricStorage(builtinStorage),
    shell_operator.WithHookMetricStorage(hookStorage),
)
```

### 4. With Convenience Function for Both Storages
```go
import (
    "github.com/flant/shell-operator/pkg/shell-operator"
    metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

storage := metricsstorage.NewStorage("my-app")

// Set both metric storages at once
operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    config.WithMetricStorages(storage, storage), // Same storage for both
)
```

### 5. Convenience Constructor with Logger
```go
// When you primarily need to provide a logger
operator, err := shell_operator.NewShellOperatorWithLogger(ctx, logger,
    config.WithMetricStorage(storage),
)
```

### 6. Configuration Presets
```go
// Development configuration with sensible defaults
cfg := shell_operator.NewDevelopmentConfig()
operator, err := shell_operator.NewShellOperatorWithConfig(ctx, cfg)

// Production configuration
cfg := shell_operator.NewProductionConfig()
operator, err := shell_operator.NewShellOperatorWithConfig(ctx, cfg)
```

### 7. Advanced Configuration
```go
// Full control over configuration
cfg := config.NewShellOperatorConfig(
    config.WithLogger(customLogger),
    config.WithMetricStorage(metricsStorage),
    config.WithHookMetricStorage(hookMetricsStorage),
    config.WithListenAddress("0.0.0.0"),
    config.WithListenPort("9090"),
    config.WithHooksDir("/app/hooks"),
    config.WithTempDir("/tmp/shell-operator"),
)

operator, err := shell_operator.NewShellOperatorWithConfig(ctx, cfg)
```

## Available Configuration Options

### Configuration Options (for NewShellOperatorWithOptions)
All configuration options are available in the `config` package:
- `config.WithLogger(logger *log.Logger)` - Set the logger
- `config.WithMetricStorage(storage)` - Set the main metrics storage
- `config.WithHookMetricStorage(storage)` - Set the hook-specific metrics storage
- `config.WithMetricStorages(metricStorage, hookStorage)` - Set both metric storages at once
- `config.WithListenAddress(address string)` - HTTP server listen address
- `config.WithListenPort(port string)` - HTTP server listen port
- `config.WithHooksDir(dir string)` - Directory containing hooks
- `config.WithTempDir(dir string)` - Temporary directory
- `config.WithDebugUnixSocket(socket string)` - Debug unix socket path
- `config.WithDebugHttpServerAddr(addr string)` - Debug HTTP server address

### Convenience Options
- `config.WithListenConfig(address, port string)` - Set both listen address and port
- `config.WithDirectories(hooksDir, tempDir string)` - Set both hooks and temp directories
- `config.WithDebugConfig(unixSocket, httpAddr string)` - Set both debug configurations
- `config.WithMetricStorages(metricStorage, hookStorage)` - Set both metric storages

## Migration from Old API

### Before (Old Init function)
```go
// Old way - rigid initialization
operator, err := shell_operator.Init(logger, metricsStorage)
```

### After (New Options Pattern)
```go
// New way - flexible configuration options
operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    config.WithLogger(logger),
    config.WithMetricStorage(metricsStorage),
)

// Or using convenience function
operator, err := shell_operator.NewShellOperatorWithLogger(ctx, logger,
    config.WithMetricStorage(metricsStorage),
)
```

## Error Handling

All constructor functions return an error if configuration validation fails:

```go
operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    shell_operator.WithHooksDir(""), // Invalid: empty hooks directory
)
if err != nil {
    log.Fatalf("Failed to create operator: %v", err)
}
```

## Best Practices

1. **Use separate metric storage options**: `WithMetricStorage()` and `WithHookMetricStorage()` for explicit control
2. **Use convenience functions when appropriate**: `WithMetricStorages()` when both storages are the same
3. **Use configuration options for complex setups**: When you need multiple configuration parameters
4. **Use presets for common scenarios**: Development vs Production configurations  
5. **Always handle errors**: Constructor functions validate configuration and can fail
6. **Prefer explicit options**: Makes configuration clear and maintainable

This API provides backward compatibility while enabling flexible, maintainable configuration patterns.
