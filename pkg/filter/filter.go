package filter

import "github.com/itchyny/gojq"

type Filter interface {
	ApplyFilter(filterStr *gojq.Code, data map[string]any) ([]byte, error)
	FilterInfo() string
}
