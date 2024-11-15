package bindingcontext

import (
	"encoding/json"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/admission/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	htypes "github.com/flant/shell-operator/pkg/hook/types"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// BindingContext contains information about event for hook
type BindingContext struct {
	Metadata struct {
		Version             string
		BindingType         htypes.BindingType
		JqFilter            string
		IncludeSnapshots    []string
		IncludeAllSnapshots bool
		Group               string
	}

	// name of a binding or a group or kubeEventType if binding has no 'name' field
	Binding string
	// additional fields for 'kubernetes' binding
	Type             kemtypes.KubeEventType
	WatchEvent       kemtypes.WatchEventType
	Objects          []kemtypes.ObjectAndFilterResult
	Snapshots        map[string][]kemtypes.ObjectAndFilterResult
	AdmissionReview  *v1.AdmissionReview
	ConversionReview *apixv1.ConversionReview
	FromVersion      string
	ToVersion        string
}

func (bc BindingContext) IsSynchronization() bool {
	return bc.Metadata.BindingType == htypes.OnKubernetesEvent && bc.Type == kemtypes.TypeSynchronization
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
	res := make(map[string]interface{})
	res["binding"] = bc.Binding

	if bc.Metadata.BindingType == htypes.OnStartup {
		return res
	}

	// Set "snapshots" field if needed.
	if len(bc.Metadata.IncludeSnapshots) > 0 || bc.Metadata.IncludeAllSnapshots {
		if len(bc.Snapshots) > 0 {
			res["snapshots"] = bc.Snapshots
		} else {
			res["snapshots"] = map[string]string{}
		}
	}

	// Handle admission and conversion before grouping.
	if bc.Metadata.BindingType == htypes.KubernetesValidating {
		res["type"] = "Validating"
		res["review"] = bc.AdmissionReview
		return res
	}

	if bc.Metadata.BindingType == htypes.KubernetesMutating {
		res["type"] = "Mutating"
		res["review"] = bc.AdmissionReview
		return res
	}

	if bc.Metadata.BindingType == htypes.KubernetesConversion {
		res["type"] = "Conversion"
		res["fromVersion"] = bc.FromVersion
		res["toVersion"] = bc.ToVersion
		res["review"] = bc.ConversionReview
		return res
	}

	// Group is always has "type: Group", even for Synchronization.
	if bc.Metadata.Group != "" {
		res["type"] = "Group"
		res["groupName"] = bc.Metadata.Group
		return res
	}

	if bc.Metadata.BindingType == htypes.Schedule {
		res["type"] = "Schedule"
		return res
	}

	// A short way for addon-operator's hooks.
	if bc.Metadata.BindingType != htypes.OnKubernetesEvent || bc.Type == "" {
		return res
	}

	// So, this BindingContext is for "kubernetes" binding.
	res["type"] = bc.Type
	// omitempty for watchEvent
	if bc.WatchEvent != "" {
		res["watchEvent"] = string(bc.WatchEvent)
	}
	switch bc.Type {
	case kemtypes.TypeSynchronization:
		if len(bc.Objects) == 0 {
			res["objects"] = make([]string, 0)
		} else {
			res["objects"] = bc.Objects
		}
	case kemtypes.TypeEvent:
		if len(bc.Objects) == 0 {
			res["object"] = nil
			if bc.Metadata.JqFilter != "" {
				res["filterResult"] = ""
			}
		} else {
			// Copy object and filterResult from the first item.
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
	res := make(map[string]interface{})
	res["binding"] = bc.Binding
	if bc.Metadata.BindingType != htypes.OnKubernetesEvent {
		return res
	}

	eventV0 := ""
	switch bc.WatchEvent {
	case kemtypes.WatchEventAdded:
		eventV0 = "add"
	case kemtypes.WatchEventModified:
		eventV0 = "update"
	case kemtypes.WatchEventDeleted:
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
