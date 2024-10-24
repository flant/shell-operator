package unilogger

import (
	"fmt"
	"log/slog"
)

var _ slog.Leveler = (*Level)(nil)

type Level slog.Level

const (
	LevelTrace Level = -8
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)

func (l Level) Level() slog.Level {
	return slog.Level(l)
}

func (l Level) String() string {
	str := func(base string, val Level) string {
		if val == 0 {
			return base
		}
		return fmt.Sprintf("%s%+d", base, val)
	}

	switch {
	case l < LevelDebug:
		return str("trace", l-LevelTrace)
	case l < LevelInfo:
		return str("debug", l-LevelDebug)
	case l < LevelWarn:
		return str("info", l-LevelInfo)
	case l < LevelError:
		return str("warn", l-LevelWarn)
	case l < LevelFatal:
		return str("error", l-LevelError)
	default:
		return str("fatal", l-LevelFatal)
	}
}
