//go:build test
// +build test

package utils

import (
	"encoding/json"
)

type JsonLogRecord map[string]string

func NewJsonLogRecord() JsonLogRecord {
	return make(JsonLogRecord)
}

func (j JsonLogRecord) FromString(s string) (JsonLogRecord, error) {
	return j.FromBytes([]byte(s))
}

func (j JsonLogRecord) FromBytes(arr []byte) (JsonLogRecord, error) {
	err := json.Unmarshal(arr, &j)
	if err != nil {
		return j, err
	}
	return j, nil
}

func (j JsonLogRecord) Map() map[string]string {
	return j
}

func (j JsonLogRecord) HasField(key string) bool {
	return HasField(j, key)
}

func (j JsonLogRecord) FieldContains(key string, s string) bool {
	return FieldContains(j, key, s)
}

func (j JsonLogRecord) FieldHasPrefix(key string, s string) bool {
	return FieldHasPrefix(j, key, s)
}

func (j JsonLogRecord) FieldEquals(key string, s string) bool {
	return FieldEquals(j, key, s)
}
