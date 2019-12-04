// +build test

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MapAsserts(t *testing.T) {
	m := map[string]string{
		"foo": "bar",
		"baz": "quux",
	}

	assert.True(t, HasField(m, "foo"))
	assert.True(t, HasField(m, "baz"))
	assert.False(t, HasField(m, "bar"))

	assert.True(t, FieldEquals(m, "foo", "bar"))
	assert.False(t, FieldEquals(m, "foo", "quux"))

	assert.True(t, FieldContains(m, "baz", "uu"))
	assert.False(t, FieldContains(m, "baz", "ZZ"))

	assert.True(t, FieldHasPrefix(m, "baz", "qu"))
	assert.False(t, FieldHasPrefix(m, "baz", "uux"))
}
