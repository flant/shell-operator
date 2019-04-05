package hook

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHook_SafeName(t *testing.T) {
	WorkingDir = "/hooks"
	hookPath := "/hooks/002-cool-hooks/monitor-namespaces.py"

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		t.Error(err)
	}

	h := NewHook(hookName, hookPath)

	assert.Equal(t, "002-cool-hooks-monitor-namespaces-py", h.SafeName())
}
