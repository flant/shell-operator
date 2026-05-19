package app

import (
"testing"

"github.com/stretchr/testify/assert"
)

func TestDebugKeepTempFiles_DefaultIsFalse(t *testing.T) {
	cfg := NewConfig()
	assert.False(t, cfg.Debug.KeepTempFiles, "KeepTempFiles should default to false")
}

// TestDebugKeepTempFilesIsBool verifies the field is of type bool.
func TestDebugKeepTempFilesIsBool(t *testing.T) {
	cfg := NewConfig()

	cfg.Debug.KeepTempFiles = true
	assert.True(t, cfg.Debug.KeepTempFiles)

	cfg.Debug.KeepTempFiles = false
	assert.False(t, cfg.Debug.KeepTempFiles)
}
