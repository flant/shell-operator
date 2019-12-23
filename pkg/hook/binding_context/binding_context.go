package hook

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// Information about event for hook
type BindingContext struct {
	Metadata struct {
		Version             string
		BindingType         BindingType
		JqFilter            string
		IncludeSnapshots    []string
		IncludeAllSnapshots bool
	}

	// name of binding or kubeEventType if binding has no 'name' field
	Binding string
	// additional fields for 'kubernetes' binding
	Type       KubeEventType
	WatchEvent WatchEventType
	Objects    []ObjectAndFilterResult
	Snapshots  map[string][]ObjectAndFilterResult
}

func (bc BindingContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(bc.Map())
}

func (bc BindingContext) Map() map[string]interface{} {
	switch bc.Metadata.Version {
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

	if bc.Metadata.BindingType == OnStartup {
		return res
	}

	if len(bc.Metadata.IncludeSnapshots) > 0 || bc.Metadata.IncludeAllSnapshots {
		key := "kubernetesSnapshots"
		if bc.Metadata.BindingType == OnKubernetesEvent {
			key = "snapshots"
		}
		if len(bc.Snapshots) > 0 {
			res[key] = bc.Snapshots
		} else {
			res[key] = map[string]string{}
		}
	}

	if bc.Metadata.BindingType != OnKubernetesEvent || bc.Type == "" {
		return res
	}

	// So, this BindingContext is for "kubernetes" binding.
	res["type"] = bc.Type
	// omitempty for watchEvent
	if bc.WatchEvent != "" {
		res["watchEvent"] = string(bc.WatchEvent)
	}
	switch bc.Type {
	case TypeSynchronization:
		if len(bc.Objects) == 0 {
			res["objects"] = make([]string, 0)
		} else {
			res["objects"] = bc.Objects
		}
	case TypeEvent:
		if len(bc.Objects) == 0 {
			res["object"] = nil
			if bc.Metadata.JqFilter != "" {
				res["filterResult"] = ""
			}
		} else {
			obj := bc.Objects[0]
			objMap := obj.Map()
			for k, v := range objMap {
				res[k] = v
			}
		}
	}

	return res
}

func (bc BindingContext) MapV0() map[string]interface{} {
	res := make(map[string]interface{}, 0)
	res["binding"] = bc.Binding
	if bc.Metadata.BindingType != OnKubernetesEvent {
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

	if len(bc.Objects) > 0 {
		res["resourceNamespace"] = bc.Objects[0].Object.GetNamespace()
		res["resourceKind"] = bc.Objects[0].Object.GetKind()
		res["resourceName"] = bc.Objects[0].Object.GetName()
	}

	return res
}

type BindingContextList []map[string]interface{}

func ConvertBindingContextList(version string, contexts []BindingContext) BindingContextList {
	res := make([]map[string]interface{}, len(contexts))
	for i, context := range contexts {
		context.Metadata.Version = version
		res[i] = context.Map()
	}
	return res
}

func (b BindingContextList) Json() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	return data, err
}
