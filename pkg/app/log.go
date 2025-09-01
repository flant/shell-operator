package app

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/config"
)

// Use info level with timestamps and a text output by default
var (
	LogLevel         = "info"
	LogNoTime        = false
	LogType          = "text"
	LogProxyHookJSON = false
)

// ForcedDurationForDebugLevel - force expiration for debug level.
const (
	ForcedDurationForDebugLevel = 30 * time.Minute
	ProxyJsonLogKey             = "proxyJsonLog"
)

// DefineLoggingFlags defines flags for logger settings.
func DefineLoggingFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("log-level", "Logging level: debug, info, error. Default is info. Can be set with $LOG_LEVEL.").
		Envar("LOG_LEVEL").
		Default(LogLevel).
		StringVar(&LogLevel)
	cmd.Flag("log-type", "Logging formatter type: json, text or color. Default is text. Can be set with $LOG_TYPE.").
		Envar("LOG_TYPE").
		Default(LogType).
		StringVar(&LogType)
	cmd.Flag("log-no-time", "Disable timestamp logging if flag is present. Useful when output is redirected to logging system that already adds timestamps. Can be set with $LOG_NO_TIME.").
		Envar("LOG_NO_TIME").
		BoolVar(&LogNoTime)
	cmd.Flag("log-proxy-hook-json", "Delegate hook stdout/ stderr JSON logging to the hooks and act as a proxy that adds some extra fields before just printing the output").
		Envar("LOG_PROXY_HOOK_JSON").
		BoolVar(&LogProxyHookJSON)
}

// SetupLogging sets logger formatter and level.
func SetupLogging(runtimeConfig *config.Config, logger *log.Logger) {
	// TODO: if we need formatters - add to logger
	// jsonFormatter := log.JSONFormatter{DisableTimestamp: LogNoTime}
	// textFormatter := log.TextFormatter{DisableTimestamp: LogNoTime, DisableColors: true}
	// colorFormatter := log.TextFormatter{DisableTimestamp: LogNoTime, ForceColors: true, FullTimestamp: true}
	// switch strings.ToLower(LogType) {
	// case "json":
	// 	log.SetFormatter(&jsonFormatter)
	// case "text":
	// 	log.SetFormatter(&textFormatter)
	// case "color":
	// 	log.SetFormatter(&colorFormatter)
	// default:
	// 	log.SetFormatter(&jsonFormatter)
	// }
	// if LogProxyHookJSON {
	// 	formatter := log.StandardLogger().Formatter
	// 	log.SetFormatter(&ProxyJsonWrapperFormatter{WrappedFormatter: formatter})
	// }

	log.SetDefaultLevel(log.LogLevelFromStr(LogLevel))

	runtimeConfig.Register("log.level",
		fmt.Sprintf("Global log level. Default duration for debug level is %s", ForcedDurationForDebugLevel),
		strings.ToLower(LogLevel),
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

// type ProxyJsonWrapperFormatter struct {
// 	WrappedFormatter log.Formatter
// }

// func (f *ProxyJsonWrapperFormatter) Format(entry *log.Entry) ([]byte, error) {
// 	// if proxying the json message is intended, just return the bytes
// 	// TODO: Find a more elegant way to carry this info
// 	if entry.Data[ProxyJsonLogKey] == true {
// 		b := bytes.NewBufferString(entry.Message)
// 		b.WriteString("\n")
// 		return b.Bytes(), nil
// 	}

// 	// otherwise, use the wrapped formatter
// 	return f.WrappedFormatter.Format(entry)
// }
