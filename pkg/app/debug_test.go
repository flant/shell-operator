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

// TestApplyConfig_OverridesDebugUnixSocket guarantees that handing a *Config
// built by an outer program to ApplyConfig replaces the package-level
// DebugUnixSocket global, even when BindFlags is never called.
func TestApplyConfig_OverridesDebugUnixSocket(t *testing.T) {
	prev := DebugUnixSocket
	t.Cleanup(func() { DebugUnixSocket = prev })

	cfg := NewConfig()
	cfg.Debug.UnixSocket = "/run/outer-program/debug.socket"

	ApplyConfig(cfg)

	assert.Equal(t, "/run/outer-program/debug.socket", DebugUnixSocket)
}

// TestApplyConfig_NilIsNoop documents that ApplyConfig tolerates a nil cfg
// so callers don't need to guard at every call site.
func TestApplyConfig_NilIsNoop(t *testing.T) {
	prev := DebugUnixSocket
	t.Cleanup(func() { DebugUnixSocket = prev })

	DebugUnixSocket = "/run/sentinel.socket"
	ApplyConfig(nil)

	assert.Equal(t, "/run/sentinel.socket", DebugUnixSocket)
}
