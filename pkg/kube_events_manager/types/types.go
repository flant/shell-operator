package types

import (
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

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
		JqFilter   string
		Checksum   string
		ResourceId string
	}
	Object       *unstructured.Unstructured // here is a pointer because of MarshalJSON receiver
	FilterResult string
}

func (o ObjectAndFilterResult) Map() map[string]interface{} {
	m := map[string]interface{}{
		"object": o.Object,
	}
	if o.Metadata.JqFilter == "" {
		return m
	}

	// Add filterResult field only if it was requested
	inJson := o.FilterResult
	if inJson == "" {
		m["filterResult"] = nil
		return m
	}
	var res interface{}
	err := json.Unmarshal([]byte(inJson), &res)
	if err != nil {
		log.Errorf("Possible bug!!! Cannot unmarshal jq filter '%s' result: %s", o.Metadata.JqFilter, err)
		m["filterResult"] = nil
		return m
	}
	m["filterResult"] = res

	return m
}

func (o ObjectAndFilterResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Map())
}

// ByNamespaceAndName implements sort.Interface for []ObjectAndFilterResult
// based on Namespace and Name of Object field.
type ByNamespaceAndName []ObjectAndFilterResult

func (a ByNamespaceAndName) Len() int      { return len(a) }
func (a ByNamespaceAndName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByNamespaceAndName) Less(i, j int) bool {
	p, q := &a[i], &a[j]
	if p.Object == nil || q.Object == nil {
		return false
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
	msgs := []string{}
	switch k.Type {
	case TypeSynchronization:
		if len(k.Objects) > 0 {
			kind := k.Objects[0].Object.GetKind()
			msgs = append(msgs, fmt.Sprintf("Synchronization with %d objects of kind '%s'", len(k.Objects), kind))
		} else {
			msgs = append(msgs, fmt.Sprintf("Synchronization with 0 objects"))
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
					msgs = append(msgs, fmt.Sprintf("Event with no kubernetes event type"))
				}
			}
		} else {
			if len(k.WatchEvents) > 0 {
				msgs = append(msgs, fmt.Sprintf("Event '%s' without objects", k.WatchEvents[0]))
			} else {
				msgs = append(msgs, fmt.Sprintf("Event without objects and kubernetes event type"))
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
