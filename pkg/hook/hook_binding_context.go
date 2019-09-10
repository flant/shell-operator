package hook

import "github.com/flant/shell-operator/pkg/kube_events_manager"

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

func ConvertBindingContextList(version string, contexts []BindingContext) interface{} {
	var versionedContext interface{}
	switch version {
	case "v0":
		versionedContext = ConvertBindingContextListV0(contexts)
	case "v1":
		versionedContext = ConvertBindingContextListV1(contexts)
	default:
		versionedContext = contexts
	}

	return versionedContext
}

func ConvertBindingContextListV0(contexts []BindingContext) []BindingContextV0 {
	res := make([]BindingContextV0, 0)
	for _, context := range contexts {
		eventV0 := ""
		switch context.WatchEvent {
		case kube_events_manager.WatchEventAdded:
			eventV0 = "add"
		case kube_events_manager.WatchEventModified:
			eventV0 = "update"
		case kube_events_manager.WatchEventDeleted:
			eventV0 = "delete"
		}
		ctx := BindingContextV0{
			Binding:           context.Binding,
			ResourceEvent:     eventV0,
			ResourceNamespace: context.Namespace,
			ResourceKind:      context.Kind,
			ResourceName:      context.Name,
		}
		res = append(res, ctx)
	}
	return res
}

func ConvertBindingContextListV1(contexts []BindingContext) []BindingContextV1 {
	res := make([]BindingContextV1, 0)
	for _, context := range contexts {
		var ctx BindingContextV1
		switch context.Type {
		case "Synchronization":
			ctx = BindingContextV1{
				Binding:      context.Binding,
				Type:         context.Type,
				Objects:      context.Objects,
				Object:       nil,
				FilterResult: "",
			}
		default:
			ctx = BindingContextV1{
				Binding:      context.Binding,
				Type:         "Event",
				WatchEvent:   string(context.WatchEvent),
				Object:       context.Object,
				FilterResult: context.FilterResult,
				Objects:      nil,
			}
		}
		res = append(res, ctx)
	}
	return res
}
