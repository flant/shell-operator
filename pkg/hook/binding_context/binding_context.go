package hook

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Information about event for hook
type BindingContext struct {
	Version     string
	BindingType BindingType

	// name of binding or kubeEventType if binding has no 'name' field
	Binding string

	// additional fields for 'kubernetes' binding
	Type         string
	WatchEvent   WatchEventType
	Object       *unstructured.Unstructured
	FilterResult string
	Objects      []ObjectAndFilterResult
	Snapshots    map[string][]ObjectAndFilterResult

	// additional field for 'schedule' binding
	KubernetesSnapshots map[string][]ObjectAndFilterResult
}

func (bc BindingContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(bc.Map())
}

func (bc BindingContext) Map() map[string]interface{} {
	switch bc.Version {
	case "v0":
		return bc.MapV0()
	case "v1":
		return bc.MapV1()
	default:
		log.Errorf("Possible bug!!! Call Map for BindingContext without version.")
		return make(map[string]interface{})
	}
}

func (bc BindingContext) MapV1() map[string]interface{} {
	res := make(map[string]interface{}, 0)
	res["binding"] = bc.Binding
	if bc.KubernetesSnapshots != nil {
		res["kubernetesSnapshots"] = bc.KubernetesSnapshots
	}
	if bc.BindingType != OnKubernetesEvent || bc.Type == "" {
		return res
	}
	// This BindingContext is for "kubernetes" binding, add more fields
	res["type"] = bc.Type
	// omitempty for watchEvent
	if bc.WatchEvent != "" {
		res["watchEvent"] = string(bc.WatchEvent)
	}
	switch bc.Type {
	case "Synchronization":
		if len(bc.Objects) == 0 {
			res["objects"] = make([]string, 0)
		} else {
			res["objects"] = bc.Objects
		}
	case "Event":
		if bc.Object != nil {
			res["object"] = bc.Object
		}
		if bc.FilterResult != "" {
			res["filterResult"] = bc.FilterResult
		}
	}
	if bc.Snapshots != nil {
		res["snapshots"] = bc.Snapshots
	}
	return res
}

func (bc BindingContext) MapV0() map[string]interface{} {
	res := make(map[string]interface{}, 0)
	res["binding"] = bc.Binding
	if bc.BindingType != OnKubernetesEvent {
		return res
	}

	eventV0 := ""
	switch bc.WatchEvent {
	case WatchEventAdded:
		eventV0 = "add"
	case WatchEventModified:
		eventV0 = "update"
	case WatchEventDeleted:
		eventV0 = "delete"
	}

	res["resourceEvent"] = eventV0

	if bc.Object != nil {
		res["resourceNamespace"] = bc.Object.GetNamespace()
		res["resourceKind"] = bc.Object.GetKind()
		res["resourceName"] = bc.Object.GetName()
	}

	return res
}

type BindingContextList []map[string]interface{}

func ConvertBindingContextList(version string, contexts []BindingContext) BindingContextList {
	res := make([]map[string]interface{}, len(contexts))
	for i, context := range contexts {
		context.Version = version
		res[i] = context.Map()
	}
	return res
}

func (b BindingContextList) Json() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	return data, err
}
