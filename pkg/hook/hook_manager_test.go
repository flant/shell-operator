package hook

import (
	"github.com/flant/shell-operator/pkg/hook/controller"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/validating_webhook"
	. "github.com/flant/shell-operator/pkg/validating_webhook/types"
)

func newHookManager(t *testing.T, testdataDir string) (*hookManager, func()) {
	var err error
	hm := NewHookManager()
	tmpDir, err := ioutil.TempDir("", "hook_manager")
	if err != nil {
		t.Fatalf("Make tmpdir should not fail: %v", err)
	}
	hooksDir, _ := filepath.Abs(testdataDir)
	hm.WithDirectories(hooksDir, tmpDir)
	hm.WithWebhookManager(validating_webhook.NewWebhookManager())
	return hm, func() { os.RemoveAll(tmpDir) }
}

func Test_HookManager_Init(t *testing.T) {
	hooksDir := "testdata/hook_manager"
	hm, rmFn := newHookManager(t, hooksDir)
	defer rmFn()

	if !strings.HasSuffix(hm.WorkingDir(), hooksDir) {
		t.Fatalf("Hook manager should has working dir '%s', got: '%s'", hooksDir, hm.WorkingDir())
	}

	err := hm.Init()
	if err != nil {
		t.Fatalf("Hook manager Init should not fail: %v", err)
	}
}

func Test_HookManager_GetHookNames(t *testing.T) {
	hm, rmFn := newHookManager(t, "testdata/hook_manager")
	defer rmFn()

	err := hm.Init()
	if err != nil {
		t.Fatalf("Hook manager Init should not fail: %v", err)
	}

	names := hm.GetHookNames()

	expectedCount := 4
	if len(names) != expectedCount {
		t.Fatalf("Hook manager should have %d hooks, got %d", expectedCount, len(names))
	}

	// TODO fix sorting!!!
	expectedNames := []string{
		"configMapHooks/hook.sh",
		"hook.sh",
		"podHooks/hook.sh",
		"podHooks/hook2.sh",
	}

	for i, expectedName := range expectedNames {
		if names[i] != expectedName {
			t.Fatalf("Hook manager should have hook '%s' at index %d, %s", expectedName, i, names[i])
		}
	}

}

func TestHookController_HandleValidatingEvent(t *testing.T) {
	g := NewWithT(t)

	hm, rmFn := newHookManager(t, "testdata/hook_manager_validating")
	defer rmFn()

	err := hm.Init()
	if err != nil {
		t.Fatalf("Hook manager Init should not fail: %v", err)
	}

	ev := ValidatingEvent{
		WebhookId:       "ololo-policy-example-com",
		ConfigurationId: "hooks",
		Review:          nil,
	}

	h := hm.GetHook("hook.sh")
	h.HookController.EnableValidatingBindings()

	canHandle := h.HookController.CanHandleValidatingEvent(ev)

	g.Expect(canHandle).To(BeTrue())

	var infoList []controller.BindingExecutionInfo
	h.HookController.HandleValidatingEvent(ev, func(info controller.BindingExecutionInfo) {
		infoList = append(infoList, info)
	})

	g.Expect(infoList).Should(HaveLen(1))

}
