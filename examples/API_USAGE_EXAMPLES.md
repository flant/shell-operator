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
)

// Using the flexible configuration system
operator, err := shell_operator.NewShellOperatorWithOptions(ctx,
    shell_operator.WithLogger(logger),
    shell_operator.WithMetricStorage(storage),
    shell_operator.WithHookMetricStorage(hookStorage), 
    shell_operator.WithHooksDir("/custom/hooks"),
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
    shell_operator.WithMetricStorages(storage, storage), // Same storage for both
)
```

### 5. Convenience Constructor with Logger
```go
// When you primarily need to provide a logger
operator, err := shell_operator.NewShellOperatorWithLogger(ctx, logger,
    shell_operator.WithMetricStorage(storage),
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
cfg := shell_operator.NewShellOperatorConfig(
    shell_operator.WithLogger(customLogger),
    shell_operator.WithMetricStorage(metricsStorage),
    shell_operator.WithHookMetricStorage(hookMetricsStorage),
    shell_operator.WithListenAddress("0.0.0.0"),
    shell_operator.WithListenPort("9090"),
    shell_operator.WithHooksDir("/app/hooks"),
    shell_operator.WithTempDir("/tmp/shell-operator"),
)

operator, err := shell_operator.NewShellOperatorWithConfig(ctx, cfg)
```

## Available Configuration Options

### Configuration Options (for NewShellOperatorWithOptions)
- `WithLogger(logger *log.Logger)` - Set the logger
- `WithMetricStorage(storage)` - Set the main metrics storage
- `WithHookMetricStorage(storage)` - Set the hook-specific metrics storage
- `WithMetricStorages(metricStorage, hookStorage)` - Set both metric storages at once
- `WithListenAddress(address string)` - HTTP server listen address
- `WithListenPort(port string)` - HTTP server listen port
- `WithHooksDir(dir string)` - Directory containing hooks
- `WithTempDir(dir string)` - Temporary directory
- `WithDebugUnixSocket(socket string)` - Debug unix socket path
- `WithDebugHttpServerAddr(addr string)` - Debug HTTP server address

### Convenience Options
- `WithListenConfig(address, port string)` - Set both listen address and port
- `WithDirectories(hooksDir, tempDir string)` - Set both hooks and temp directories
- `WithDebugConfig(unixSocket, httpAddr string)` - Set both debug configurations
- `WithMetricStorages(metricStorage, hookStorage)` - Set both metric storages

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
    shell_operator.WithLogger(logger),
    shell_operator.WithMetricStorage(metricsStorage),
)

// Or using convenience function
operator, err := shell_operator.NewShellOperatorWithLogger(ctx, logger,
    shell_operator.WithMetricStorage(metricsStorage),
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
