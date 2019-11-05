package kube_events_manager

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/kube"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

// ResourceInformer is a kube informer for particular onKubernetesEvent
type ResourceInformer interface {
	WithNamespace(string)
	WithName(string)
	CreateSharedInformer() error
	GetExistedObjects() []ObjectAndFilterResult
	Run(stopCh <-chan struct{})
	Stop()
}

type resourceInformer struct {
	Monitor *MonitorConfig
	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string

	Checksum       map[string]string
	ExistedObjects []ObjectAndFilterResult
	SharedInformer cache.SharedInformer

	GroupVersionResource schema.GroupVersionResource
	ListOptions          metav1.ListOptions

	ctx context.Context
}

// resourceInformer should implement ResourceInformer
var _ ResourceInformer = &resourceInformer{}

var NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
	informer := &resourceInformer{
		Monitor:        monitor,
		Checksum:       make(map[string]string),
		ExistedObjects: make([]ObjectAndFilterResult, 0),
	}
	return informer
}

func (ei *resourceInformer) WithNamespace(ns string) {
	ei.Namespace = ns
}

func (ei *resourceInformer) WithName(name string) {
	ei.Name = name
}

func (ei *resourceInformer) CreateSharedInformer() (err error) {
	// discover GroupVersionResource for informer
	log.Debugf("%s: discover GVR for apiVersion '%s' kind '%s'...", ei.Monitor.Metadata.DebugName, ei.Monitor.ApiVersion, ei.Monitor.Kind)
	ei.GroupVersionResource, err = kube.GroupVersionResource(ei.Monitor.ApiVersion, ei.Monitor.Kind)
	if err != nil {
		log.Errorf("%s: Cannot get GroupVersionResource info for apiVersion '%s' kind '%s' from api-server. Possibly CRD is not created before informers are started. Error was: %v", ei.Monitor.Metadata.DebugName, ei.Monitor.ApiVersion, ei.Monitor.Kind, err)
		return err
	}
	log.Debugf("%s: GVR for kind '%s' is '%s'", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, ei.GroupVersionResource.String())

	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	fmtLabelSelector, err := FormatLabelSelector(ei.Monitor.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %s", ei.Monitor.LabelSelector.String(), err)
	}

	fieldSelector := ei.adjustFieldSelector(ei.Monitor.FieldSelector, ei.Name)
	fmtFieldSelector, err := FormatFieldSelector(fieldSelector)
	if err != nil {
		return fmt.Errorf("format field selector '%+v': %s", fieldSelector, err)
	}

	tweakListOptions := func(options *metav1.ListOptions) {
		if fmtFieldSelector != "" {
			options.FieldSelector = fmtFieldSelector
		}
		if fmtLabelSelector != "" {
			options.LabelSelector = fmtLabelSelector
		}
	}
	ei.ListOptions = metav1.ListOptions{}
	tweakListOptions(&ei.ListOptions)

	// create informer with add, update, delete callbacks
	informer := dynamicinformer.NewFilteredDynamicInformer(kube.DynamicClient, ei.GroupVersionResource, ei.Namespace, resyncPeriod, indexers, tweakListOptions)
	informer.Informer().AddEventHandler(SharedInformerEventHandler(ei))
	ei.SharedInformer = informer.Informer()

	err = ei.ListExistedObjects()
	if err != nil {
		log.Errorf("list existing objects: %v", err)
		return err
	}

	return nil
}

func (ei *resourceInformer) GetExistedObjects() []ObjectAndFilterResult {
	return ei.ExistedObjects
}

var SharedInformerEventHandler = func(informer *resourceInformer) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				log.Errorf("%s: WATCH Added: get object id: %s", informer.Monitor.Metadata.DebugName, err)
				return
			}

			filtered, err := resourceFilter(obj, informer.Monitor.JqFilter)
			if err != nil {
				log.Errorf("%s: WATCH Added: apply jqFilter on %s: %s",
					informer.Monitor.Metadata.DebugName, objectId, err)
				return
			}

			checksum := utils_checksum.CalculateChecksum(filtered)

			filteredResult := ""
			if informer.Monitor.JqFilter != "" {
				filteredResult = filtered
			}

			if informer.ShouldHandleEvent(WatchEventAdded) {
				jqFilterOutput := ""
				if informer.Monitor.JqFilter != "" {
					jqFilterOutput = fmt.Sprintf(": jqFilter '%s' output:\n%s",
						informer.Monitor.JqFilter,
						utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}
				log.Debugf("%s: WATCH Added: %s%s",
					informer.Monitor.Metadata.DebugName,
					objectId,
					jqFilterOutput)
				informer.HandleKubeEvent(obj, objectId, filteredResult, checksum, WatchEventAdded)
			}
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				log.Errorf("%s: WATCH Modified: get object id: %s", informer.Monitor.Metadata.DebugName, err)
				return
			}

			filtered, err := resourceFilter(obj, informer.Monitor.JqFilter)
			if err != nil {
				log.Errorf("%s: WATCH Modified: apply jqFilter on %s: %s",
					informer.Monitor.Metadata.DebugName, objectId, err)
				return
			}

			checksum := utils_checksum.CalculateChecksum(filtered)

			filteredResult := ""
			if informer.Monitor.JqFilter != "" {
				filteredResult = filtered
			}

			if informer.ShouldHandleEvent(WatchEventModified) {
				jqFilterOutput := ""
				if informer.Monitor.JqFilter != "" {
					jqFilterOutput = fmt.Sprintf(": jqFilter '%s' output:\n%s",
						informer.Monitor.JqFilter,
						utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}
				log.Debugf("%s: WATCH Modified: %s object%s",
					informer.Monitor.Metadata.DebugName,
					objectId,
					jqFilterOutput)
				informer.HandleKubeEvent(obj, objectId, filteredResult, checksum, WatchEventModified)
			}

		},
		DeleteFunc: func(obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				log.Errorf("%s: WATCH Deleted: get object id: %s", informer.Monitor.Metadata.DebugName, err)
				return
			}

			filtered, err := resourceFilter(obj, informer.Monitor.JqFilter)
			if err != nil {
				log.Errorf("%s: WATCH Modified: apply jqFilter on %s: %s",
					informer.Monitor.Metadata.DebugName, objectId, err)
				return
			}

			filteredResult := ""
			if informer.Monitor.JqFilter != "" {
				filteredResult = filtered
			}

			if informer.ShouldHandleEvent(WatchEventDeleted) {
				jqFilterOutput := ""
				if informer.Monitor.JqFilter != "" {
					jqFilterOutput = fmt.Sprintf(": jqFilter '%s' output:\n%s",
						informer.Monitor.JqFilter,
						utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}
				log.Debugf("%s: WATCH Deleted: %s object%s",
					informer.Monitor.Metadata.DebugName,
					objectId,
					jqFilterOutput)
				informer.HandleKubeEvent(obj, objectId, filteredResult, "deleted", WatchEventDeleted)
			}
		},
	}
}

// ListExistedObjects get a list of existed objects in namespace that match selectors and
// fills Checksum map with checksums of existing objects.
func (ei *resourceInformer) ListExistedObjects() error {
	objList, err := kube.DynamicClient.
		Resource(ei.GroupVersionResource).
		Namespace(ei.Namespace).
		List(ei.ListOptions)
	if err != nil {
		log.Errorf("%s: initial list resources of kind '%s': %v", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, err)
		return err
	}

	if objList == nil || len(objList.Items) == 0 {
		log.Debugf("%s: Got no existing '%s' resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind)
		return nil
	}

	log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)

	for _, obj := range objList.Items {
		resourceId, err := runtimeResourceId(&obj, ei.Monitor.Kind)
		if err != nil {
			return err
		}

		filtered, err := resourceFilter(obj.Object, ei.Monitor.JqFilter)
		if err != nil {
			return err
		}

		ei.Checksum[resourceId] = utils_checksum.CalculateChecksum(filtered)

		jqFilterOutput := ""
		if ei.Monitor.JqFilter != "" {
			jqFilterOutput = fmt.Sprintf(" jqFilter '%s' output:\n%s",
				ei.Monitor.JqFilter,
				utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
		}
		log.Debugf("%s: initial checksum of %s is %s.%s",
			ei.Monitor.Metadata.DebugName,
			resourceId,
			ei.Checksum[resourceId],
			jqFilterOutput)

		filterResult := ""
		if ei.Monitor.JqFilter != "" {
			filterResult = filtered
		}

		ei.ExistedObjects = append(ei.ExistedObjects, ObjectAndFilterResult{
			Object:       obj.Object,
			FilterResult: filterResult,
		})
	}

	return nil
}

// HandleKubeEvent sends new KubeEvent to KubeEventCh
// obj doesn't contains Kind information, so kind is passed from AddMonitor() argument.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modifed is enough)
func (ei *resourceInformer) HandleKubeEvent(obj interface{}, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
	if ei.Checksum[objectId] != newChecksum {
		ei.Checksum[objectId] = newChecksum

		log.Debugf("%s: %+v %s: checksum changed, send KubeEvent",
			ei.Monitor.Metadata.DebugName,
			string(eventType),
			objectId,
		)
		// Safe to ignore an error because of previous call to runtimeResourceId()
		namespace, name, _ := metaFromEventObject(obj.(runtime.Object))

		log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)

		var eventObj map[string]interface{}
		switch v := obj.(type) {
		case unstructured.Unstructured:
			eventObj = v.Object
		case *unstructured.Unstructured:
			eventObj = v.Object
		default:
			eventObj = map[string]interface{}{
				"object": obj,
			}
		}

		KubeEventCh <- KubeEvent{
			ConfigId:     ei.Monitor.Metadata.ConfigId,
			Type:         "Event",
			WatchEvents:  []WatchEventType{eventType},
			Namespace:    namespace,
			Kind:         ei.Monitor.Kind,
			Name:         name,
			Object:       eventObj,
			FilterResult: filterResult,
		}
	} else {
		log.Debugf("%s: %+v %s: checksum is not changed",
			ei.Monitor.Metadata.DebugName,
			string(eventType),
			objectId,
		)
	}

	return
}

func (ei *resourceInformer) adjustFieldSelector(selector *FieldSelector, objName string) *FieldSelector {
	var selectorCopy *FieldSelector

	if selector != nil {
		selectorCopy = &FieldSelector{
			MatchExpressions: selector.MatchExpressions,
		}
	}

	if objName != "" {
		objNameReq := FieldSelectorRequirement{
			Field:    "metadata.name",
			Operator: "=",
			Value:    objName,
		}
		if selectorCopy == nil {
			selectorCopy = &FieldSelector{
				MatchExpressions: []FieldSelectorRequirement{
					objNameReq,
				},
			}
		} else {
			selectorCopy.MatchExpressions = append(selectorCopy.MatchExpressions, objNameReq)
		}
	}

	return selectorCopy
}

func (ei *resourceInformer) ShouldHandleEvent(checkEvent WatchEventType) bool {
	for _, event := range ei.Monitor.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *resourceInformer) Run(stopCh <-chan struct{}) {
	log.Debugf("%s: RUN resource informer", ei.Monitor.Metadata.DebugName)
	ei.SharedInformer.Run(stopCh)
}

func (ei *resourceInformer) Stop() {
	log.Debugf("%s: STOP resource informer", ei.Monitor.Metadata.DebugName)
}
