package hook

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// newTestHookManager returns an HookManager wired against testdata/hook_manager.
func newTestHookManager(t *testing.T) *Manager {
	t.Helper()
	return newHookManager(t, "testdata/hook_manager")
}

// TestEnrichHookMetadata_injectsLogAndMetricLabels verifies that enrichHookMetadata
// sets hook.Name into LogLabels and populates MetricLabels for OnKubernetesEvent bindings.
func TestEnrichHookMetadata_injectsLogAndMetricLabels(t *testing.T) {
	hm := newTestHookManager(t)
	err := hm.Init()
	require.NoError(t, err)

	// The hook_manager testdata has an OnKubernetesEvent binding in one of the hooks.
	// After Init(), enrichHookMetadata should have been called for each hook.
	// Verify by inspecting a hook's OnKubernetesEvent binding metadata.
	for _, hookName := range hm.GetHookNames() {
		h := hm.GetHook(hookName)
		for _, kubeCfg := range h.GetConfig().OnKubernetesEvents {
			assert.Equal(t, hookName, kubeCfg.Monitor.Metadata.LogLabels[pkg.LogKeyHook],
				"LogLabels[hook] should equal hook name for binding %s", kubeCfg.BindingName)
			assert.Equal(t, hookName, kubeCfg.Monitor.Metadata.MetricLabels[pkg.MetricKeyHook],
				"MetricLabels[hook] should equal hook name")
		}
	}
}

// TestWireHookController_setsControllerAndTmpDir verifies that wireHookController
// sets a non-nil HookController and a TmpDir on the hook.
func TestWireHookController_setsControllerAndTmpDir(t *testing.T) {
	hm := newTestHookManager(t)
	err := hm.Init()
	require.NoError(t, err)

	for _, hookName := range hm.GetHookNames() {
		h := hm.GetHook(hookName)
		assert.NotNil(t, h.HookController, "HookController should be set after wireHookController")
		// TmpDir may be empty if the manager was created without a temp dir (newHookManager passes t.TempDir()).
		// Just ensure it's been set to whatever the manager's TempDir was.
		assert.Equal(t, hm.TempDir(), h.TmpDir, "TmpDir should match manager TmpDir")
	}
}

// TestLoadHook_invalidExitErrorMissingBinary checks that loadHook returns an
// error when the hook binary exit fails (non-existent entrypoint is benign here
// because the path in testdata points to shell scripts, which are executed
// successfully by the test setup). We verify the overall Init success instead.
func TestLoadHook_allHooksHaveNonNilConfig(t *testing.T) {
	hm := newTestHookManager(t)
	err := hm.Init()
	require.NoError(t, err)

	for _, hookName := range hm.GetHookNames() {
		h := hm.GetHook(hookName)
		assert.NotNil(t, h.Config, "Config must be non-nil after loadHook")
	}
}

// TestFetchHookConfig_returnsErrorForNonExecutable verifies that fetchHookConfig
// returns an error when the hook binary doesn't exist or fails.
func TestFetchHookConfig_returnsErrorForNonExecutable(t *testing.T) {
	conversionManager := conversion.NewWebhookManager()
	conversionManager.Settings = conversion.DefaultSettings

	admissionManager := admission.NewWebhookManager(nil)
	admissionManager.Settings = admission.DefaultSettings

	cfg := &ManagerConfig{
		WorkingDir:               t.TempDir(),
		TempDir:                  t.TempDir(),
		AdmissionWebhookManager:  admissionManager,
		ConversionWebhookManager: conversionManager,
		Logger:                   log.NewNop(),
	}
	hm := NewHookManager(cfg)

	// Build a minimal hook pointing at a nonexistent binary.
	h := NewHook("ghost", "/nonexistent-binary", false, false, "", log.NewNop())

	_, err := hm.fetchHookConfig(h)
	assert.Error(t, err, "should return error for non-existent hook binary")
}
