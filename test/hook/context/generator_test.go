package context

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
)

func parseContexts(contexts string) []BindingContext {
	parsedBindingContexts := []BindingContext{}
	json.Unmarshal([]byte(contexts), &parsedBindingContexts)
	return parsedBindingContexts
}

func Test_BindingContextGenerator(t *testing.T) {
	t.Run("Binding context generator test", func(t *testing.T) {
		c, err := NewBindingContextController(`
configVersion: v1
kubernetes:
- apiVersion: v1
  includeSnapshotsFrom:
  - selected_pods
  kind: Pod
  name: selected_pods
schedule:
- name: every_minute
  crontab: "* * * * *"
  includeSnapshotsFrom:
  - selected_pods
`, `
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
`)
		assert.Equal(t, err, nil)

		// Synchronization contexts
		contexts, err := c.Run()
		assert.Equal(t, err, nil)

		parsedBindingContexts := parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].Type), "Synchronization")
		assert.Equal(t, len(parsedBindingContexts[0].Objects), 2)
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)

		// Object added
		contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
---
apiVersion: v1
kind: Pod
metadata:
  name: pod3
spec:
  containers: []
`)
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Added")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 3)

		// Object modified
		contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
---
apiVersion: v1
kind: Pod
metadata:
  name: pod3
spec:
  containers:
  - name: test
`)
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Modified")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 3)

		// Object deleted
		contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
`)
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Deleted")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)

		// Run schedule
		contexts, err = c.RunSchedule("* * * * *")
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)
	})
}
