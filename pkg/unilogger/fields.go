package unilogger

import (
	"encoding/json"
	"log/slog"

	"gopkg.in/yaml.v3"
)

type formatter string

const (
	jsonFormatter = "json"
	yamlFormatter = "yaml"
)

var _ slog.LogValuer = (*Raw)(nil)

func RawJSON(key, text string) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: NewJSONRaw(text).LogValue(),
	}
}

func RawYAML(key, text string) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: NewYAMLRaw(text).LogValue(),
	}
}

// made them public to use without slog.Attr
func NewJSONRaw(text string) *Raw {
	return &Raw{
		formatter: jsonFormatter,
		text:      text,
	}
}

func NewYAMLRaw(text string) *Raw {
	return &Raw{
		formatter: yamlFormatter,
		text:      text,
	}
}

type Raw struct {
	formatter formatter
	text      string
}

func (r *Raw) LogValue() slog.Value {
	raw := make(map[string]any, 1)

	switch r.formatter {
	case jsonFormatter:
		if err := json.Unmarshal([]byte(r.text), &raw); err == nil {
			return slog.AnyValue(raw)
		}
	case yamlFormatter:
		if err := yaml.Unmarshal([]byte(r.text), &raw); err == nil {
			return slog.AnyValue(raw)
		}
	}

	return slog.StringValue(r.text)
}
