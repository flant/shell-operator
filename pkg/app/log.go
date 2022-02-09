package app

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Use info level with timestamps and a text output by default
var LogLevel = "info"
var LogNoTime = false
var LogType = "text"

// SetupLoggingSettings init global flags for logging
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

// SetupLogging sets logging output
func SetupLogging() {
	switch LogType {
	case "json":
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, DisableColors: true})
	case "color":
		log.SetFormatter(&log.TextFormatter{DisableTimestamp: LogNoTime, ForceColors: true, FullTimestamp: true})
	default:
		log.SetFormatter(&log.JSONFormatter{DisableTimestamp: LogNoTime})
	}

	switch strings.ToLower(LogLevel) {
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
