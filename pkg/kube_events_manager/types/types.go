package types

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type WatchEventType string

const (
	WatchEventAdded    WatchEventType = "Added"
	WatchEventModified WatchEventType = "Modified"
	WatchEventDeleted  WatchEventType = "Deleted"
	Synchronization    WatchEventType = "Synchronization"
)

type KubeEventType string

const (
	TypeSynchronization KubeEventType = "Synchronization"
	TypeEvent           KubeEventType = "Event"
)

// TODO remove this type with cleanup of v0.
type KubeEventMode string

const (
	ModeV0          KubeEventMode = "v0"          // No first Synchronization, only Event.
	ModeIncremental KubeEventMode = "Incremental" // Send Synchronization with existed object and Event for each followed event.
)

type ObjectAndFilterResult struct {
	Object       *unstructured.Unstructured `json:"object,omitempty"`
	FilterResult string                     `json:"filterResult,omitempty"`
}

// KubeEvent contains MonitorId from monitor configuration, event type
// and involved k8s objects.
type KubeEvent struct {
	MonitorId    string
	Type         string // Event or Synchronization
	WatchEvents  []WatchEventType
	Object       *unstructured.Unstructured // for Event
	FilterResult string                     // for Event
	Objects      []ObjectAndFilterResult    // for Synchronization
}

func (k KubeEvent) String() string {
	msgs := []string{}
	switch k.Type {
	case "Synchronization":
		if len(k.Objects) > 0 {
			kind := k.Objects[0].Object.GetKind()
			msgs = append(msgs, fmt.Sprintf("Synchronization with %d objects of kind '%s'", len(k.Objects), kind))
		} else {
			msgs = append(msgs, fmt.Sprintf("Synchronization with 0 objects"))
		}
	case "Event":
		if len(k.WatchEvents) > 0 {
			if k.Object != nil {
				msgs = append(msgs, fmt.Sprintf("Event '%s' for %s/%s/%s", k.WatchEvents[0], k.Object.GetNamespace(), k.Object.GetKind(), k.Object.GetName()))
			} else {
				msgs = append(msgs, fmt.Sprintf("Event '%s' without object", k.WatchEvents[0]))
			}
		} else {
			if k.Object != nil {
				msgs = append(msgs, fmt.Sprintf("Event with no kubernetes event type for %s/%s/%s", k.Object.GetNamespace(), k.Object.GetKind(), k.Object.GetName()))
			} else {
				msgs = append(msgs, fmt.Sprintf("Event with no kubernetes event type"))
			}
		}
	default:
		msgs = append(msgs, fmt.Sprintf("unknown type: '%s' from monitor '%s'", k.Type, k.MonitorId))
	}
	return strings.Join(msgs, " ")
}

// Monitor configuration

type NameSelector struct {
	MatchNames []string `json:"matchNames"`
}

type FieldSelectorRequirement struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

type FieldSelector struct {
	MatchExpressions []FieldSelectorRequirement `json:"matchExpressions"`
}

type NamespaceSelector struct {
	NameSelector  *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}
