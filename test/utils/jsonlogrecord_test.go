//go:build test
// +build test

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_JsonLogRecord(t *testing.T) {
	line := "{\"foo\":\"bar\", \"baz\":\"qrux\" }"

	logRecord, err := NewJsonLogRecord().FromString(line)
	assert.NoError(t, err)

	assert.True(t, logRecord.HasField("foo"))
	assert.True(t, logRecord.FieldEquals("foo", "bar"))
	assert.False(t, logRecord.FieldEquals("foo", "Bar"))
	assert.True(t, logRecord.HasField("baz"))
	assert.True(t, logRecord.FieldContains("baz", "ux"))
	assert.False(t, logRecord.FieldContains("baz", "zz"))

	m := map[string]string{
		"foo": "bar",
		"baz": "qrux",
	}
	assert.Equal(t, m, logRecord.Map())
}
