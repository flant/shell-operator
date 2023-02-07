//go:build test
// +build test

package utils

import "strings"

func HasField(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

func FieldContains(m map[string]string, key string, s string) bool {
	v, ok := m[key]
	return ok && strings.Contains(v, s)
}

func FieldHasPrefix(m map[string]string, key string, s string) bool {
	v, ok := m[key]
	return ok && strings.HasPrefix(v, s)
}

func FieldEquals(m map[string]string, key string, s string) bool {
	v, ok := m[key]
	return ok && v == s
}
