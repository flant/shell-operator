package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubConverter is a VersionedConverter that tracks call counts.
type stubConverter struct {
	version   string
	callCount int
	errReturn error
}

func (s *stubConverter) Version() string { return s.version }

func (s *stubConverter) ConvertAndCheck(_ []byte, _ *HookConfig) error {
	s.callCount++
	return s.errReturn
}

func TestVersionedConverterRegistry_knownVersionsExist(t *testing.T) {
	for _, ver := range []string{"v0", "v1"} {
		_, ok := versionedConverters[ver]
		assert.True(t, ok, "registry should have a converter for version %s", ver)
	}
}

func TestVersionedConverterRegistry_unsupportedVersionReturnsError(t *testing.T) {
	cfg := &HookConfig{Version: "vX-unknown"}
	err := cfg.ConvertAndCheck([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestVersionedConverterRegistry_RegisterAndUse(t *testing.T) {
	const testVer = "vTEST-internal"
	stub := &stubConverter{version: testVer}

	// Register into the global registry.
	RegisterVersionedConverter(stub)
	t.Cleanup(func() { delete(versionedConverters, testVer) })

	cfg := &HookConfig{Version: testVer}
	err := cfg.ConvertAndCheck([]byte(`{}`))

	assert.NoError(t, err)
	assert.Equal(t, 1, stub.callCount, "converter should be called exactly once")
}

func TestVersionedConverterRegistry_RegisterError(t *testing.T) {
	const testVer = "vTEST-err"
	stub := &stubConverter{version: testVer, errReturn: errTest}

	RegisterVersionedConverter(stub)
	t.Cleanup(func() { delete(versionedConverters, testVer) })

	cfg := &HookConfig{Version: testVer}
	err := cfg.ConvertAndCheck([]byte(`{}`))
	assert.ErrorIs(t, err, errTest)
}

var errTest = &testError{"converter error"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestHookConfigV0Converter_implementsVersionedConverter(t *testing.T) {
	var _ VersionedConverter = hookConfigV0Converter{}
}

func TestHookConfigV1Converter_implementsVersionedConverter(t *testing.T) {
	var _ VersionedConverter = hookConfigV1Converter{}
}
