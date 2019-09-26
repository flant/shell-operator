package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetSchema(t *testing.T) {
	schemas := []string{"v0", "v1"}

	for _, schema := range schemas {
		s := GetSchema(schema)
		assert.NotNil(t, s)
	}
}

func Test_LoadSchema_From_Schemas(t *testing.T) {
	for _, schema := range Schemas {
		s, err := LoadSchema(schema)
		if assert.NoError(t, err) {
			assert.NotNil(t, s)
		}
	}
}
