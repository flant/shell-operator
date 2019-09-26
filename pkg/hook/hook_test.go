package hook

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Hook_SafeName(t *testing.T) {
	WorkingDir := "/hooks"
	hookPath := "/hooks/002-cool-hooks/monitor-namespaces.py"

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		t.Error(err)
	}

	h := NewHook(hookName, hookPath)

	assert.Equal(t, "002-cool-hooks-monitor-namespaces-py", h.SafeName())
}

func Test_Hook_WithConfig(t *testing.T) {
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
				if assert.NoError(t, err) {
					assert.NotNil(t, hook.Config)
					assert.Equal(t, []BindingType{OnStartup}, hook.Config.Bindings())
					assert.NotNil(t, hook.Config.OnStartup)
					assert.Equal(t, 10.0, hook.Config.OnStartup.Order)
				}
			},
		},
		{
			"with validation error",
			`{"configVersion":"v1", "onStartup": "10"}`,
			func() {
				assert.Error(t, err)
				t.Logf("expected validation error was: %v", err)
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
