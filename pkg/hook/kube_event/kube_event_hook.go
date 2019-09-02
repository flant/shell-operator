package kube_event

// a link between a hook and a kube event informer
type KubeEventHook struct {
	// hook name
	HookName string

	ConfigName   string
	AllowFailure bool
}
