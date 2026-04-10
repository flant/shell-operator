package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileSystemHookDiscovery_Discover_emptyDir returns no paths for an empty directory.
func TestFileSystemHookDiscovery_Discover_emptyDir(t *testing.T) {
	dir := t.TempDir()
	d := FileSystemHookDiscovery{}
	paths, err := d.Discover(dir)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

// TestFileSystemHookDiscovery_Discover_findsExecutables returns executable files.
func TestFileSystemHookDiscovery_Discover_findsExecutables(t *testing.T) {
	dir := t.TempDir()

	// Create an executable file.
	execPath := filepath.Join(dir, "my-hook.sh")
	err := os.WriteFile(execPath, []byte("#!/bin/sh\necho ok\n"), 0o755)
	require.NoError(t, err)

	d := FileSystemHookDiscovery{}
	paths, err := d.Discover(dir)
	require.NoError(t, err)
	assert.Contains(t, paths, execPath)
}

// TestFileSystemHookDiscovery_Discover_ignoresNonExecutables skips regular files.
func TestFileSystemHookDiscovery_Discover_ignoresNonExecutables(t *testing.T) {
	dir := t.TempDir()

	// Non-executable file.
	nonExecPath := filepath.Join(dir, "README.md")
	err := os.WriteFile(nonExecPath, []byte("docs"), 0o644)
	require.NoError(t, err)

	d := FileSystemHookDiscovery{}
	paths, err := d.Discover(dir)
	require.NoError(t, err)
	assert.NotContains(t, paths, nonExecPath)
}

// TestFileSystemHookDiscovery_Discover_recursive finds executables in subdirectories.
func TestFileSystemHookDiscovery_Discover_recursive(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	hookPath := filepath.Join(subdir, "hook.sh")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/sh\n"), 0o755))

	d := FileSystemHookDiscovery{}
	paths, err := d.Discover(dir)
	require.NoError(t, err)
	assert.Contains(t, paths, hookPath)
}

// TestNewHookManager_defaultDiscovery verifies that a nil HookDiscovery in
// ManagerConfig defaults to FileSystemHookDiscovery inside the Manager.
func TestNewHookManager_defaultDiscovery(t *testing.T) {
	hm := newHookManager(t, t.TempDir())
	hm.hookDiscovery = nil // reset to exercise the nil-default path via NewHookManager
	cfg := &ManagerConfig{
		WorkingDir:               t.TempDir(),
		TempDir:                  t.TempDir(),
		HookDiscovery:            nil, // explicitly nil → should default
		AdmissionWebhookManager:  hm.admissionWebhookManager,
		ConversionWebhookManager: hm.conversionWebhookManager,
		Logger:                   hm.logger,
	}
	hm2 := NewHookManager(cfg)
	require.NotNil(t, hm2)
	assert.IsType(t, FileSystemHookDiscovery{}, hm2.hookDiscovery)
}

// TestNewHookManager_injectedDiscovery verifies that a custom HookDiscovery is
// stored as-is in the Manager.
func TestNewHookManager_injectedDiscovery(t *testing.T) {
	stub := &stubDiscovery{}
	hm := newHookManager(t, t.TempDir())
	cfg := &ManagerConfig{
		WorkingDir:               t.TempDir(),
		TempDir:                  t.TempDir(),
		HookDiscovery:            stub,
		AdmissionWebhookManager:  hm.admissionWebhookManager,
		ConversionWebhookManager: hm.conversionWebhookManager,
		Logger:                   hm.logger,
	}
	hm2 := NewHookManager(cfg)
	assert.Equal(t, stub, hm2.hookDiscovery)
}

// TestManager_Init_usesInjectedDiscovery verifies that Manager.Init calls
// HookDiscovery.Discover rather than the filesystem when a custom discovery is
// injected.  An empty result means Init succeeds with zero hooks loaded.
func TestManager_Init_usesInjectedDiscovery(t *testing.T) {
	stub := &stubDiscovery{paths: []string{}} // returns nothing
	hm := newHookManager(t, t.TempDir())
	hm.hookDiscovery = stub

	err := hm.Init()
	require.NoError(t, err)
	assert.True(t, stub.called, "Discover should have been called")
	assert.Equal(t, []string{}, hm.GetHookNames())
}

// TestManager_Init_discoveryError propagates discovery errors.
func TestManager_Init_discoveryError(t *testing.T) {
	stub := &stubDiscovery{err: errStubDiscovery}
	hm := newHookManager(t, t.TempDir())
	hm.hookDiscovery = stub

	err := hm.Init()
	require.Error(t, err)
	assert.ErrorIs(t, err, errStubDiscovery)
}

// ---- helpers ----

var errStubDiscovery = &testError{"stub discovery error"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

type stubDiscovery struct {
	paths  []string
	err    error
	called bool
}

func (s *stubDiscovery) Discover(_ string) ([]string, error) {
	s.called = true
	return s.paths, s.err
}
