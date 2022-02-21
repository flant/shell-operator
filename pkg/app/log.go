package app

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/config"
)

// Use info level with timestamps and a text output by default
var LogLevel = "info"
var LogNoTime = false
var LogType = "text"

// ForcedDurationForDebugLevel - force expiration for debug level.
const ForcedDurationForDebugLevel = 30 * time.Minute

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
}

// SetupLogging sets logger formatter and level.
func SetupLogging(runtimeConfig *config.Config) {
	switch strings.ToLower(LogType) {
	case "json":
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, DisableColors: true})
	case "color":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, ForceColors: true, FullTimestamp: true})
	default:
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	}

	setLogLevel(LogLevel)

	runtimeConfig.Register("log.level",
		fmt.Sprintf("Global log level. Default duration for debug level is %s", ForcedDurationForDebugLevel),
		strings.ToLower(LogLevel),
		func(oldValue string, newValue string) error {
			log.Infof("Set log level to '%s'", newValue)
			setLogLevel(newValue)
			return nil
		}, func(oldValue string, newValue string) time.Duration {
			if strings.ToLower(newValue) == "debug" {
				return ForcedDurationForDebugLevel
			}
			return 0
		})
}

func setLogLevel(logLevel string) {
	switch strings.ToLower(logLevel) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
