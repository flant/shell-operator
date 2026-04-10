package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDebugKeepTmpFiles_DefaultIsFalse(t *testing.T) {
	assert.False(t, DebugKeepTmpFiles, "DebugKeepTmpFiles should default to false")
}

// TestDebugKeepTmpFilesIsBool verifies the variable is the bool type
// and that simple assignment behaves correctly (no string comparison needed).
func TestDebugKeepTmpFilesIsBool(t *testing.T) {
	saved := DebugKeepTmpFiles
	defer func() { DebugKeepTmpFiles = saved }()

	DebugKeepTmpFiles = true
	assert.True(t, DebugKeepTmpFiles)

	DebugKeepTmpFiles = false
	assert.False(t, DebugKeepTmpFiles)
}
