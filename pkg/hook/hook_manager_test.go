package hook

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/validating"
	. "github.com/flant/shell-operator/pkg/webhook/validating/types"
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
	conversionManager := conversion.NewWebhookManager()
	conversionManager.Settings = app.ConversionWebhookSettings
	hm.WithConversionWebhookManager(conversionManager)
	validatingManager := validating.NewWebhookManager()
	validatingManager.Settings = app.ValidatingWebhookSettings
	hm.WithValidatingWebhookManager(validatingManager)

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
		WebhookId:       "test-policy-example-com",
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

func Test_HookManager_conversion_chains(t *testing.T) {
	g := NewWithT(t)

	hm, rmFn := newHookManager(t, "testdata/hook_manager_conversion_chains")
	defer rmFn()

	err := hm.Init()
	g.Expect(err).ShouldNot(HaveOccurred(), "Hook manager Init should not fail: %v", err)

	g.Expect(hm.conversionChains.Chains).Should(HaveLen(2), "There should be conversion chains for 2 CRDs.")

	crdName := "crontabs.stable.example.com"
	g.Expect(hm.conversionChains.Chains).Should(HaveKey(crdName))

	chain := hm.conversionChains.Get(crdName)
	// 6 paths in cache for each binding.
	g.Expect(chain.PathsCache).Should(HaveLen(6))

	var convPath []conversion.Rule

	// Find path for unknown crd
	convPath = hm.FindConversionChain("unknown"+crdName, conversion.Rule{
		FromVersion: "alpha",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown from version
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "unknown-version",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown to version
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "beta",
		ToVersion:   "unknown-version",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown from and to versions
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "from-unknown-version",
		ToVersion:   "to-unknown-version",
	})
	g.Expect(convPath).Should(BeNil())

	// Find a simple path.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "alpha",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(HaveLen(1))

	// Find a full path in an "up" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "alpha",
		ToVersion:   "delta",
	})
	g.Expect(convPath).Should(HaveLen(3))
	g.Expect(convPath[0].String()).Should(Equal("alpha->beta"))
	g.Expect(convPath[1].String()).Should(Equal("beta->gamma"))
	g.Expect(convPath[2].String()).Should(Equal("gamma->delta"))

	// Find a full path in a "down" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "delta",
		ToVersion:   "alpha",
	})
	g.Expect(convPath).Should(HaveLen(3))
	g.Expect(convPath[0].String()).Should(Equal("delta->gamma"))
	g.Expect(convPath[1].String()).Should(Equal("gamma->beta"))
	g.Expect(convPath[2].String()).Should(Equal("beta->alpha"))

	// Find a part path in an "up" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "beta",
		ToVersion:   "delta",
	})
	g.Expect(convPath).Should(HaveLen(2))
	g.Expect(convPath[0].String()).Should(Equal("beta->gamma"))
	g.Expect(convPath[1].String()).Should(Equal("gamma->delta"))

	// Find a part path in a "down" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "gamma",
		ToVersion:   "alpha",
	})
	g.Expect(convPath).Should(HaveLen(2))
	g.Expect(convPath[0].String()).Should(Equal("gamma->beta"))
	g.Expect(convPath[1].String()).Should(Equal("beta->alpha"))

	// Cache has 6 paths from bindings, 2 more paths for each full path and 1 more path for each part path.
	g.Expect(chain.PathsCache).Should(HaveLen(6 + 2 + 2 + 1 + 1))
}

func Test_HookManager_conversion_chains_full(t *testing.T) {
	g := NewWithT(t)

	hm, rmFn := newHookManager(t, "testdata/hook_manager_conversion_chains_full")
	defer rmFn()

	err := hm.Init()
	g.Expect(err).ShouldNot(HaveOccurred(), "Hook manager Init should not fail: %v", err)

	g.Expect(hm.conversionChains.Chains).Should(HaveLen(2), "There should be conversion chains for 2 CRDs.")

	crdName := "crontabs.stable.example.com"
	g.Expect(hm.conversionChains.Chains).Should(HaveKey(crdName))

	chain := hm.conversionChains.Get(crdName)
	// 6 paths in cache for each binding.
	g.Expect(chain.PathsCache).Should(HaveLen(6))

	var convPath []conversion.Rule

	// Find path for unknown crd
	convPath = hm.FindConversionChain("unknown"+crdName, conversion.Rule{
		FromVersion: "alpha",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown from version
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "unknown-version",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown to version
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "beta",
		ToVersion:   "unknown-version",
	})
	g.Expect(convPath).Should(BeNil())

	// Find path for unknown from and to versions
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "from-unknown-version",
		ToVersion:   "to-unknown-version",
	})
	g.Expect(convPath).Should(BeNil())

	// Find a simple path.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "alpha",
		ToVersion:   "beta",
	})
	g.Expect(convPath).Should(HaveLen(1))

	// Find a simple path with full versions.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "group.io/alpha",
		ToVersion:   "unstable.example.com/beta",
	})
	g.Expect(convPath).Should(HaveLen(1))

	// Find a full path in an "up" direction with full versions.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "group.io/alpha",
		ToVersion:   "delta",
	})
	// g.Expect(hm.conversionChains["crontabs.stable.example.com"]).Should(HaveLen(1))
	g.Expect(convPath).Should(HaveLen(3))
	g.Expect(convPath[0].String()).Should(Equal("group.io/alpha->beta"))
	g.Expect(convPath[1].String()).Should(Equal("unstable.example.com/beta->gamma"))
	g.Expect(convPath[2].String()).Should(Equal("stable.example.com/gamma->next.io/delta"))

	// Find a full path in a "down" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "delta",
		ToVersion:   "alpha",
	})
	g.Expect(convPath).Should(HaveLen(3))
	g.Expect(convPath[0].String()).Should(Equal("stable.example.com/delta->stable.example.com/gamma"))
	g.Expect(convPath[1].String()).Should(Equal("gamma->beta"))
	g.Expect(convPath[2].String()).Should(Equal("beta->alpha"))

	// Find a part path in an "up" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "beta",
		ToVersion:   "delta",
	})
	g.Expect(convPath).Should(HaveLen(2))
	g.Expect(convPath[0].String()).Should(Equal("unstable.example.com/beta->gamma"))
	g.Expect(convPath[1].String()).Should(Equal("stable.example.com/gamma->next.io/delta"))

	// Find a part path in a "down" direction.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "gamma",
		ToVersion:   "alpha",
	})
	g.Expect(convPath).Should(HaveLen(2))
	g.Expect(convPath[0].String()).Should(Equal("gamma->beta"))
	g.Expect(convPath[1].String()).Should(Equal("beta->alpha"))

	// Cache has 6 paths from bindings, 2 more paths for each full path and 1 more path for each part path.
	g.Expect(chain.PathsCache).Should(HaveLen(6+2+2+1+1), "PathCache should contain only paths from hook, no additional paths with short versions are allowed.")

	// Check a 'different group but same version' conversion
	crdName = "crontabs.unstable.example.com"
	g.Expect(hm.conversionChains.Chains).Should(HaveKey(crdName))

	chain = hm.conversionChains.Get(crdName)
	// 3 paths in cache for hook2.sh.
	g.Expect(chain.PathsCache).Should(HaveLen(3))

	// Find a same version path.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "v1beta1",
		ToVersion:   "v1beta1",
	})
	g.Expect(convPath).Should(HaveLen(1))
	g.Expect(convPath[0].String()).Should(Equal("alpha.example.com/v1beta1->alpha.example.io/v1beta1"))

	// Find a same version path with full versions.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "alpha.example.com/v1beta1",
		ToVersion:   "alpha.example.io/v1beta1",
	})
	g.Expect(convPath).Should(HaveLen(1))
	g.Expect(convPath[0].String()).Should(Equal("alpha.example.com/v1beta1->alpha.example.io/v1beta1"))

	// Find a same version path with full versions.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "v1beta1",
		ToVersion:   "alpha.example.io/v1beta1",
	})
	g.Expect(convPath).Should(HaveLen(1))
	g.Expect(convPath[0].String()).Should(Equal("alpha.example.com/v1beta1->alpha.example.io/v1beta1"))

	// Find a same version path with full versions.
	convPath = hm.FindConversionChain(crdName, conversion.Rule{
		FromVersion: "alpha.example.com/v1beta1",
		ToVersion:   "v1beta1",
	})
	g.Expect(convPath).Should(HaveLen(1))
	g.Expect(convPath[0].String()).Should(Equal("alpha.example.com/v1beta1->alpha.example.io/v1beta1"))
}
