package app

import (
"fmt"
"log/slog"
"strings"
"time"

"github.com/deckhouse/deckhouse/pkg/log"
)

// ForcedDurationForDebugLevel - force expiration for debug level.
const (
ForcedDurationForDebugLevel = 30 * time.Minute
ProxyJsonLogKey             = "proxyJsonLog"
)

type Registerer interface {
	Register(key string, help string, defaultValue string,
setter func(key string, newValue string) error,
expirer func(key string, newValue string) time.Duration)
}

// SetupLogging sets the log level and registers a runtime config hook so the
// level can be changed without restarting the operator.
func SetupLogging(level string, runtimeConfig Registerer, logger *log.Logger) {
	log.SetDefaultLevel(log.LogLevelFromStr(level))

	runtimeConfig.Register("log.level",
fmt.Sprintf("Global log level. Default duration for debug level is %s", ForcedDurationForDebugLevel),
strings.ToLower(level),
func(_ string, newValue string) error {
logger.Info("Change log level", slog.String("value", newValue))
log.SetDefaultLevel(log.LogLevelFromStr(newValue))
return nil
}, func(_ string, newValue string) time.Duration {
if strings.ToLower(newValue) == "debug" {
return ForcedDurationForDebugLevel
}
return 0
})
}
