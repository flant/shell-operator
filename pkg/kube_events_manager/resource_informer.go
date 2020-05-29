package kube_events_manager

import (
	"context"
	"fmt"
	"runtime/trace"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metrics_storage"
	. "github.com/flant/shell-operator/pkg/utils/measure"
)

// ResourceInformer is a kube informer for particular onKubernetesEvent
type ResourceInformer interface {
	WithContext(ctx context.Context)
	WithKubeClient(client kube.KubernetesClient)
	WithMetricStorage(mstor *metrics_storage.MetricStorage)
	WithNamespace(string)
	WithName(string)
	WithKubeEventCb(eventCb func(KubeEvent))
	CreateSharedInformer() error
	GetExistedObjects() []ObjectAndFilterResult
	Start()
	Stop()
	PauseHandleEvents()
}

type resourceInformer struct {
	KubeClient kube.KubernetesClient
	Monitor    *MonitorConfig
	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string

	CachedObjects  map[string]*ObjectAndFilterResult
	cacheLock      sync.RWMutex
	SharedInformer cache.SharedInformer

	GroupVersionResource schema.GroupVersionResource
	ListOptions          metav1.ListOptions

	eventCb func(KubeEvent)

	// TODO resourceInformer should be stoppable (think of deleted namespaces and disabled modules in addon-operator)
	ctx    context.Context
	cancel context.CancelFunc

	metricStorage *metrics_storage.MetricStorage

	// a flag to stop handle events after Stop()
	stopped bool
}

// resourceInformer should implement ResourceInformer
var _ ResourceInformer = &resourceInformer{}

var NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
	informer := &resourceInformer{
		Monitor:       monitor,
		CachedObjects: make(map[string]*ObjectAndFilterResult),
		cacheLock:     sync.RWMutex{},
	}
	return informer
}

func (ei *resourceInformer) WithContext(ctx context.Context) {
	ei.ctx, ei.cancel = context.WithCancel(ctx)
}

func (ei *resourceInformer) WithKubeClient(client kube.KubernetesClient) {
	ei.KubeClient = client
}

func (ei *resourceInformer) WithMetricStorage(mstor *metrics_storage.MetricStorage) {
	ei.metricStorage = mstor
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
	ei.GroupVersionResource, err = ei.KubeClient.GroupVersionResource(ei.Monitor.ApiVersion, ei.Monitor.Kind)
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
	informer := dynamicinformer.NewFilteredDynamicInformer(ei.KubeClient.Dynamic(), ei.GroupVersionResource, ei.Namespace, resyncPeriod, indexers, tweakListOptions)
	informer.Informer().AddEventHandler(ei)
	ei.SharedInformer = informer.Informer()

	err = ei.ListExistedObjects()
	if err != nil {
		log.Errorf("list existing objects: %v", err)
		return err
	}

	return nil
}

// TODO we need locks between HandleEvent and GetExistedObjects
func (ei *resourceInformer) GetExistedObjects() []ObjectAndFilterResult {
	ei.cacheLock.RLock()
	defer ei.cacheLock.RUnlock()
	res := make([]ObjectAndFilterResult, 0)
	for _, obj := range ei.CachedObjects {
		res = append(res, *obj)
	}
	return res
}

// ListExistedObjects get a list of existed objects in namespace that match selectors and
// fills Checksum map with checksums of existing objects.
func (ei *resourceInformer) ListExistedObjects() error {
	defer trace.StartRegion(context.Background(), "ListExistedObjects").End()
	objList, err := ei.KubeClient.Dynamic().
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

	// FIXME objList.Items has too much information for log
	//log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)
	log.Debugf("%s: '%s' initial list: Got %d existing resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, len(objList.Items))

	var filteredObjects = make(map[string]*ObjectAndFilterResult)

	for _, item := range objList.Items {
		// copy loop var to avoid duplication of pointer
		obj := item
		//objFilterRes, err := ApplyJqFilter(ei.Monitor.JqFilter, &obj)

		var objFilterRes *ObjectAndFilterResult
		var err error
		func() {
			defer MeasureTime(func(nanos Nanos) {
				ei.metricStorage.ObserveHistogram("kube_jq_hist", nanos.Ms(), ei.Monitor.Metadata.MetricLabels)
			})()
			objFilterRes, err = ApplyJqFilter(ei.Monitor.JqFilter, &obj)
		}()

		if err != nil {
			return err
		}
		// save object to the cache

		filteredObjects[objFilterRes.Metadata.ResourceId] = objFilterRes

		log.Debugf("%s: initial list: '%s' is cached with checksum %s",
			ei.Monitor.Metadata.DebugName,
			objFilterRes.Metadata.ResourceId,
			objFilterRes.Metadata.Checksum)
	}

	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	for k, v := range filteredObjects {
		ei.CachedObjects[k] = v
	}

	return nil
}

func (ei *resourceInformer) OnAdd(obj interface{}) {
	ei.HandleWatchEvent(obj, WatchEventAdded)
}

func (ei *resourceInformer) OnUpdate(oldObj, newObj interface{}) {
	ei.HandleWatchEvent(newObj, WatchEventModified)
}

func (ei *resourceInformer) OnDelete(obj interface{}) {
	ei.HandleWatchEvent(obj, WatchEventDeleted)
}

// HandleKubeEvent register object in cache. Pass object to callback if object's checksum is changed.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modified is enough)
//func (ei *resourceInformer) HandleKubeEvent(obj *unstructured.Unstructured, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
func (ei *resourceInformer) HandleWatchEvent(object interface{}, eventType WatchEventType) {
	// check if stop
	if ei.stopped {
		return
	}

	defer MeasureTime(func(nanos Nanos) {
		ei.metricStorage.ObserveHistogram("kube_event_duration_hist", nanos.Ms(), ei.Monitor.Metadata.MetricLabels)
	})()
	defer trace.StartRegion(context.Background(), "HandleWatchEvent").End()

	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	var obj = object.(*unstructured.Unstructured)

	resourceId := ResourceId(obj)

	// Always calculate checksum and update cache, because we need actual state in CachedObjects

	var objFilterRes *ObjectAndFilterResult
	var err error
	func() {
		defer MeasureTime(func(nanos Nanos) {
			ei.metricStorage.ObserveHistogram("kube_jq_hist", nanos.Ms(), ei.Monitor.Metadata.MetricLabels)
		})()
		objFilterRes, err = ApplyJqFilter(ei.Monitor.JqFilter, obj)
	}()
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
		// Update object in cache
		ei.cacheLock.Lock()
		cachedObject, objectInCache := ei.CachedObjects[resourceId]
		skipEvent := false
		if objectInCache && cachedObject.Metadata.Checksum == objFilterRes.Metadata.Checksum {
			// update object in cache and do not send event
			log.Debugf("%s: %s %s: checksum is not changed, no KubeEvent",
				ei.Monitor.Metadata.DebugName,
				string(eventType),
				resourceId,
			)
			skipEvent = true
		}
		ei.CachedObjects[resourceId] = objFilterRes
		ei.cacheLock.Unlock()
		if skipEvent {
			return
		}

	case WatchEventDeleted:
		ei.cacheLock.Lock()
		delete(ei.CachedObjects, resourceId)
		ei.cacheLock.Unlock()
	}

	// Fire KubeEvent only if needed.
	if ei.ShouldFireEvent(eventType) {
		log.Debugf("%s: %s %s: send KubeEvent",
			ei.Monitor.Metadata.DebugName,
			string(eventType),
			resourceId,
		)
		// TODO: should be disabled by default and enabled by a debug feature switch
		//log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)

		// Pass event info to callback
		ei.EventCb(KubeEvent{
			MonitorId:   ei.Monitor.Metadata.MonitorId,
			WatchEvents: []WatchEventType{eventType},
			Objects:     []ObjectAndFilterResult{*objFilterRes},
		})
	}
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

func (ei *resourceInformer) ShouldFireEvent(checkEvent WatchEventType) bool {
	for _, event := range ei.Monitor.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *resourceInformer) Start() {
	log.Debugf("%s: RUN resource informer", ei.Monitor.Metadata.DebugName)
	stopCh := make(chan struct{}, 1)
	go func() {
		<-ei.ctx.Done()
		ei.stopped = true
		close(stopCh)
	}()

	ei.SharedInformer.Run(stopCh)
}

func (ei *resourceInformer) Stop() {
	log.Debugf("%s: STOP resource informer", ei.Monitor.Metadata.DebugName)
	if ei.cancel != nil {
		ei.cancel()
	}
	ei.stopped = true
}

func (ei *resourceInformer) PauseHandleEvents() {
	log.Debugf("%s: PAUSE resource informer", ei.Monitor.Metadata.DebugName)
	ei.stopped = true
}
