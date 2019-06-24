package kube_event

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

func Test_MakeKubeEventHookDescriptors(t *testing.T) {
	hookSh := &hook.Hook{
		Name:     "hook",
		Path:     "/hooks/hook.sh",
		Bindings: []hook.BindingType{},
		Config: &hook.HookConfig{
			OnKubernetesEvent: []kube_events_manager.OnKubernetesEventConfig{
				{Kind: "pod", NamespaceSelector: &kube_events_manager.KubeNamespaceSelector{Any: true}},
			},
		},
	}

	evs := MakeKubeEventHookDescriptors(hookSh)

	assert.Equal(t, len(evs), 1)
}

func Test_MakeKubeEventHookDescriptors_Two(t *testing.T) {
	hookSh := hook.NewHook("hook2", "/hooks/hook-2.sh")
	config := `{"onStartup": 10,
  "onKubernetesEvent":[
  {"kind": "pod",
   "event":["add"],
   "namespaceSelector": {"matchNames":["example-monitor-pods"]}
  },
  {"kind": "Crontab",
   "event":["add", "update"]
  }
]}
`
	h, err := hookSh.WithConfig([]byte(config))

	assert.NoError(t, err)

	assert.Equal(t, len(h.Config.OnKubernetesEvent), 2)

	evs := MakeKubeEventHookDescriptors(h)

	assert.Equal(t, len(evs), 2)
}
