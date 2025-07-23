package types

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type WatchEventType string

const (
	WatchEventAdded    WatchEventType = "Added"
	WatchEventModified WatchEventType = "Modified"
	WatchEventDeleted  WatchEventType = "Deleted"
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
	Metadata struct {
		JqFilter     string
		Checksum     uint64
		ResourceId   string // Used for sorting
		RemoveObject bool
	}
	Object       *unstructured.Unstructured // here is a pointer because of MarshalJSON receiver
	FilterResult interface{}
}

// Map constructs a map suitable for use in binding context.
func (o ObjectAndFilterResult) Map() map[string]interface{} {
	m := map[string]interface{}{}

	if !o.Metadata.RemoveObject {
		m["object"] = o.Object
	}

	if o.Metadata.JqFilter == "" && o.FilterResult == nil {
		// No jqFilter, no filterResult -> filterResult field should not be in a map.
		return m
	}

	var filterResultValue interface{}
	if o.Metadata.JqFilter != "" {
		// jqFilter is set, so filterResult field should be in a map.
		// FilterResult is a jq output and should be a string.
		filterResString, ok := o.FilterResult.(string)
		if !ok || filterResString == "" {
			m["filterResult"] = nil
			return m
		}

		// Convert string with jq output into Go object.
		err := json.Unmarshal([]byte(filterResString), &filterResultValue)
		if err != nil {
			log.Error("Possible bug!!! Cannot unmarshal jq filter result",
				slog.String("jqFilter", o.Metadata.JqFilter),
				log.Err(err))
			m["filterResult"] = nil
			return m
		}
	} else {
		filterResultValue = o.FilterResult
	}

	m["filterResult"] = filterResultValue

	return m
}

func (o ObjectAndFilterResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Map())
}

func (o *ObjectAndFilterResult) RemoveFullObject() {
	o.Object = nil
	o.Metadata.RemoveObject = true
}

type ObjectAndFilterResults map[string]*ObjectAndFilterResult

// ByNamespaceAndName implements sort.Interface for []ObjectAndFilterResult
// based on Namespace and Name of Object field.
// TODO use special fields instead of ResourceId. ResourceId can be changed in the future.
type ByNamespaceAndName []ObjectAndFilterResult

func (a ByNamespaceAndName) Len() int      { return len(a) }
func (a ByNamespaceAndName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByNamespaceAndName) Less(i, j int) bool {
	p, q := &a[i], &a[j]
	if p.Object == nil || q.Object == nil {
		return p.Metadata.ResourceId < q.Metadata.ResourceId
	}
	switch {
	case p.Object.GetNamespace() < q.Object.GetNamespace():
		return true
	case p.Object.GetNamespace() > q.Object.GetNamespace():
		return false
	}
	// Namespaces are equal, so compare names.
	return p.Object.GetName() < q.Object.GetName()
}

// KubeEvent contains MonitorId from monitor configuration, event type
// and involved k8s objects.
type KubeEvent struct {
	MonitorId   string
	Type        KubeEventType // Event or Synchronization
	WatchEvents []WatchEventType
	Objects     []ObjectAndFilterResult
}

func (k KubeEvent) String() string {
	msgs := make([]string, 0)
	switch k.Type {
	case TypeSynchronization:
		if len(k.Objects) > 0 {
			kind := k.Objects[0].Object.GetKind()
			msgs = append(msgs, fmt.Sprintf("Synchronization with %d objects of kind '%s'", len(k.Objects), kind))
		} else {
			msgs = append(msgs, "Synchronization with 0 objects")
		}
	case TypeEvent:
		if len(k.Objects) == 1 {
			obj := k.Objects[0].Object
			if len(k.WatchEvents) > 0 {
				if obj != nil {
					msgs = append(msgs, fmt.Sprintf("Event '%s' for %s/%s/%s", k.WatchEvents[0], obj.GetNamespace(), obj.GetKind(), obj.GetName()))
				} else {
					msgs = append(msgs, fmt.Sprintf("Event '%s' without object", k.WatchEvents[0]))
				}
			} else {
				if obj != nil {
					msgs = append(msgs, fmt.Sprintf("Event with no kubernetes event type for %s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName()))
				} else {
					msgs = append(msgs, "Event with no kubernetes event type")
				}
			}
		} else {
			if len(k.WatchEvents) > 0 {
				msgs = append(msgs, fmt.Sprintf("Event '%s' without objects", k.WatchEvents[0]))
			} else {
				msgs = append(msgs, "Event without objects and kubernetes event type")
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
