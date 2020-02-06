package hook

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/types"
)

func Test_Hook_SafeName(t *testing.T) {
	g := NewWithT(t)

	WorkingDir := "/hooks"
	hookPath := "/hooks/002-cool-hooks/monitor-namespaces.py"

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		t.Error(err)
	}

	h := NewHook(hookName, hookPath)

	g.Expect(h.SafeName()).To(Equal("002-cool-hooks-monitor-namespaces-py"))
}

func Test_Hook_WithConfig(t *testing.T) {
	g := NewWithT(t)

	var hook *Hook
	var err error

	tests := []struct {
		name     string
		jsonData string
		fn       func()
	}{
		{
			"simple",
			`{"onStartup": 10}`,
			func() {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(hook.Config).ToNot(BeNil())
				g.Expect(hook.Config.Bindings()).To(Equal([]BindingType{OnStartup}))
				g.Expect(hook.Config.OnStartup).ToNot(BeNil())
				g.Expect(hook.Config.OnStartup.Order).To(Equal(10.0))
			},
		},
		{
			"with validation error",
			`{"configVersion":"v1", "onStartup": "10"}`,
			func() {
				g.Expect(err).Should(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("onStartup must be of type integer: \"string\""))

				//t.Logf("expected validation error was: %v", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hook = NewHook("hook-sh", "/hooks/hook.sh")
			_, err = hook.WithConfig([]byte(test.jsonData))
			test.fn()
		})
	}
}
