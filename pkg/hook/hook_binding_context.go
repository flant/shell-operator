package hook

import (
	"encoding/json"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

// Additional info from schedule and kube events
type BindingContext struct {
	Binding string `json:"binding"`
	Type    string
	// event type from kube API
	WatchEvent kube_events_manager.WatchEventType `json:"watchEvent,omitempty"`

	Namespace string `json:"resourceNamespace,omitempty"`
	Kind      string `json:"resourceKind,omitempty"`
	Name      string `json:"resourceName,omitempty"`

	Object       map[string]interface{}
	FilterResult string

	Objects []interface{}
}

// Additional info from schedule and kube events
type BindingContextV0 struct {
	Binding string `json:"binding"`
	// lower cased event type
	ResourceEvent     string `json:"resourceEvent,omitempty"`
	ResourceNamespace string `json:"resourceNamespace,omitempty"`
	ResourceKind      string `json:"resourceKind,omitempty"`
	ResourceName      string `json:"resourceName,omitempty"`
}

// Additional info from schedule and kube events
type BindingContextV1 struct {
	Binding string `json:"binding"`
	Type    string `json:"type"`
	// event type from kube API
	WatchEvent string `json:"watchEvent,omitempty"`
	// lower cased event type
	Object       map[string]interface{} `json:"object,omitempty"`
	FilterResult string                 `json:"filterResult,omitempty"`

	Objects []interface{} `json:"objects,omitempty"`
}

func (bc *BindingContextV1) Map() map[string]interface{} {
	res := make(map[string]interface{}, 0)
	res["binding"] = bc.Binding
	if bc.Type == "" {
		return res
	}
	// This BindingContext is for "kubernetes" binding, add more fields
	res["type"] = bc.Type
	// omitempty for watchEvent
	if bc.WatchEvent != "" {
		res["watchEvent"] = bc.WatchEvent
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
	return res
}

func (bc *BindingContextV1) MarshalJSON() ([]byte, error) {
	return json.Marshal(bc.Map())
}

func ConvertBindingContext(version string, context BindingContext) interface{} {
	var versionedContext interface{}
	switch version {
	case "v0":
		versionedContext = ConvertBindingContextV0(context)
	case "v1":
		versionedContext = ConvertBindingContextV1(context)
	default:
		versionedContext = context
	}

	return versionedContext
}

func ConvertBindingContextV0(context BindingContext) BindingContextV0 {
	eventV0 := ""
	switch context.WatchEvent {
	case kube_events_manager.WatchEventAdded:
		eventV0 = "add"
	case kube_events_manager.WatchEventModified:
		eventV0 = "update"
	case kube_events_manager.WatchEventDeleted:
		eventV0 = "delete"
	}
	return BindingContextV0{
		Binding:           context.Binding,
		ResourceEvent:     eventV0,
		ResourceNamespace: context.Namespace,
		ResourceKind:      context.Kind,
		ResourceName:      context.Name,
	}
}

func ConvertBindingContextV1(context BindingContext) BindingContextV1 {
	var ctx BindingContextV1
	switch context.Type {
	case "Synchronization":
		ctx = BindingContextV1{
			Binding: context.Binding,
			Type:    context.Type,
			Objects: context.Objects,
		}
	case "Event":
		ctx = BindingContextV1{
			Binding:      context.Binding,
			Type:         context.Type,
			WatchEvent:   string(context.WatchEvent),
			Object:       context.Object,
			FilterResult: context.FilterResult,
		}
	default:
		// non kubernetes binding
		ctx = BindingContextV1{
			Binding: context.Binding,
		}
	}
	return ctx
}

func ConvertBindingContextList(version string, contexts []BindingContext) interface{} {
	res := make([]interface{}, len(contexts))
	for i, context := range contexts {
		res[i] = ConvertBindingContext(version, context)
	}
	return res
}

func ConvertBindingContextListV0(contexts []BindingContext) []BindingContextV0 {
	res := make([]BindingContextV0, len(contexts))
	for i, context := range contexts {
		res[i] = ConvertBindingContextV0(context)
	}
	return res
}

func ConvertBindingContextListV1(contexts []BindingContext) []BindingContextV1 {
	res := make([]BindingContextV1, len(contexts))
	for i, context := range contexts {
		res[i] = ConvertBindingContextV1(context)
	}
	return res
}
