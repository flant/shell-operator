package kube_events_manager

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/kube"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

// ResourceInformer is a kube informer for particular onKubernetesEvent
type ResourceInformer interface {
	WithNamespace(string)
	WithName(string)
	WithKubeEventCb(eventCb func(KubeEvent))
	CreateSharedInformer() error
	GetExistedObjects() []ObjectAndFilterResult
	Run(stopCh <-chan struct{})
	Stop()
}

type CachedObject struct {
	Checksum string
	Object   ObjectAndFilterResult
}

type resourceInformer struct {
	Monitor *MonitorConfig
	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string

	CachedObjects  map[string]*CachedObject
	SharedInformer cache.SharedInformer

	GroupVersionResource schema.GroupVersionResource
	ListOptions          metav1.ListOptions

	eventCb func(KubeEvent)

	ctx context.Context
}

// resourceInformer should implement ResourceInformer
var _ ResourceInformer = &resourceInformer{}

var NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
	informer := &resourceInformer{
		Monitor:       monitor,
		CachedObjects: make(map[string]*CachedObject, 0),
	}
	return informer
}

func (ei *resourceInformer) WithNamespace(ns string) {
	ei.Namespace = ns
}

func (ei *resourceInformer) WithName(name string) {
	ei.Name = name
}

func (ei *resourceInformer) WithKubeEventCb(eventCb func(KubeEvent)) {
	ei.eventCb = eventCb
}

func (ei *resourceInformer) EventCb(ev KubeEvent) {
	if ei.eventCb != nil {
		ei.eventCb(ev)
	}
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
	res := make([]ObjectAndFilterResult, 0)
	for _, obj := range ei.CachedObjects {
		res = append(res, obj.Object)
	}

	return res
	//	return ei.ExistedObjects
}

// TODO make monitor config accessible here to get LogLabels
var SharedInformerEventHandler = func(informer *resourceInformer) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			informer.HandleWatchEvent(obj.(*unstructured.Unstructured), WatchEventAdded)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			informer.HandleWatchEvent(obj.(*unstructured.Unstructured), WatchEventModified)
		},
		DeleteFunc: func(obj interface{}) {
			informer.HandleWatchEvent(obj.(*unstructured.Unstructured), WatchEventDeleted)
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
		resourceId := ResourceId(&obj)

		filterResult, checksum, err := ApplyJqFilter(ei.Monitor.JqFilter, &obj)
		if err != nil {
			return err
		}

		jqFilterOutput := ""
		if ei.Monitor.JqFilter != "" {
			jqFilterOutput = fmt.Sprintf(" jqFilter '%s' output:\n%s",
				ei.Monitor.JqFilter,
				utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filterResult)))
		}
		log.Debugf("%s: initial checksum of %s is %s.%s",
			ei.Monitor.Metadata.DebugName,
			resourceId,
			checksum,
			jqFilterOutput)

		// save object to cache
		ei.CachedObjects[resourceId] = &CachedObject{
			Checksum: checksum,
			Object: ObjectAndFilterResult{
				Object:       &obj,
				FilterResult: filterResult,
			},
		}
	}

	return nil
}

// HandleKubeEvent register object in cache. Pass object to callback if object's checksum is changed.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modifed is enough)
//func (ei *resourceInformer) HandleKubeEvent(obj *unstructured.Unstructured, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
func (ei *resourceInformer) HandleWatchEvent(obj *unstructured.Unstructured, eventType WatchEventType) {
	resourceId := ResourceId(obj)

	if !ei.ShouldHandleEvent(eventType) {
		// TODO it seems too easy for bindings with jqFilter

		// object should be deleted from cache even if hook is not subscribed to Delete
		if eventType == WatchEventDeleted {
			delete(ei.CachedObjects, resourceId)
		}

		return
	}

	filterResult, newChecksum, err := ApplyJqFilter(ei.Monitor.JqFilter, obj)
	if err != nil {
		log.Errorf("%s: WATCH %s: %s",
			ei.Monitor.Metadata.DebugName,
			eventType,
			err)
		return
	}

	// Ignore Added or Modified if object is in cache and its checksum is equal to the newChecksum.
	// Delete is never ignored.
	switch eventType {
	case WatchEventAdded:
		fallthrough
	case WatchEventModified:
		cachedObject, objectInCache := ei.CachedObjects[resourceId]
		if objectInCache && cachedObject.Checksum == newChecksum {
			// ignore changes
			log.Debugf("%s: %s %s: checksum is not changed, no KubeEvent",
				ei.Monitor.Metadata.DebugName,
				string(eventType),
				resourceId,
			)
			return
		} else {
			// save new object and result
			ei.CachedObjects[resourceId] = &CachedObject{
				Checksum: newChecksum,
				Object: ObjectAndFilterResult{
					Object:       obj,
					FilterResult: filterResult,
				},
			}
		}

	case WatchEventDeleted:
		delete(ei.CachedObjects, resourceId)
	}

	log.Debugf("%s: %s %s: send KubeEvent",
		ei.Monitor.Metadata.DebugName,
		string(eventType),
		resourceId,
	)
	// TODO: should be disabled by default and enabled by a debug feature switch
	//log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)

	// Pass event info to callback
	ei.EventCb(KubeEvent{
		MonitorId:    ei.Monitor.Metadata.MonitorId,
		WatchEvents:  []WatchEventType{eventType},
		Object:       obj,
		FilterResult: filterResult,
	})

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

func ApplyJqFilter(jqFilter string, obj *unstructured.Unstructured) (result string, checksum string, err error) {
	resourceId := ResourceId(obj)

	if jqFilter == "" {
		jsonObject, err := obj.MarshalJSON()
		if err != nil {
			return "", "", fmt.Errorf("calculate the object checksumm failed on %s: %s", resourceId, err)
		}
		return "", utils_checksum.CalculateChecksum(string(jsonObject)), nil
	}

	filtered, err := ResourceFilter(obj, jqFilter)
	if err != nil {
		return "", "", fmt.Errorf("apply jqFilter on %s: %s", resourceId, err)
	}

	checksum = utils_checksum.CalculateChecksum(filtered)

	return filtered, checksum, nil
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
