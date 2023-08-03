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
	"k8s.io/client-go/tools/cache"

	klient "github.com/flant/kube-client/client"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

// ResourceInformer is a kube informer for particular onKubernetesEvent
type ResourceInformer interface {
	WithContext(ctx context.Context)
	WithKubeClient(client klient.Client)
	WithMetricStorage(mstor *metric_storage.MetricStorage)
	WithNamespace(string)
	WithName(string)
	WithKubeEventCb(eventCb func(KubeEvent))
	CreateSharedInformer() error
	CachedObjects() []ObjectAndFilterResult
	EnableKubeEventCb() // Call it to use KubeEventCb to emit events.
	Start()
	Stop()
	PauseHandleEvents()
	CachedObjectsInfo() CachedObjectsInfo
	CachedObjectsInfoIncrement() CachedObjectsInfo
}

type resourceInformer struct {
	KubeClient klient.Client
	Monitor    *MonitorConfig
	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string

	// Kubernetes informer and its settings.
	FactoryIndex         FactoryIndex
	GroupVersionResource schema.GroupVersionResource
	ListOptions          metav1.ListOptions

	// A cache of objects and filterResults. It is a part of the Monitor's snapshot.
	cachedObjects map[string]*ObjectAndFilterResult
	cacheLock     sync.RWMutex

	// Cached objects operations since start
	cachedObjectsInfo *CachedObjectsInfo
	// Cached objects operations since last access
	cachedObjectsIncrement *CachedObjectsInfo

	// Events buffer for "Synchronization" mode: it stores events between CachedObjects call and EnableKubeEventCb call
	// to replay them when "Synchronization" hook is done.
	eventBuf     []KubeEvent
	eventBufLock sync.Mutex

	// A callback function that define custom handling of Kubernetes events.
	eventCb        func(KubeEvent)
	eventCbEnabled bool

	// TODO resourceInformer should be stoppable (think of deleted namespaces and disabled modules in addon-operator)
	ctx    context.Context
	cancel context.CancelFunc

	metricStorage *metric_storage.MetricStorage

	// a flag to stop handle events after Stop()
	stopped bool
}

// resourceInformer should implement ResourceInformer
var _ ResourceInformer = &resourceInformer{}

var NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
	informer := &resourceInformer{
		Monitor:                monitor,
		cachedObjects:          make(map[string]*ObjectAndFilterResult),
		cacheLock:              sync.RWMutex{},
		eventBufLock:           sync.Mutex{},
		cachedObjectsInfo:      &CachedObjectsInfo{},
		cachedObjectsIncrement: &CachedObjectsInfo{},
	}
	return informer
}

func (ei *resourceInformer) WithContext(ctx context.Context) {
	ei.ctx, ei.cancel = context.WithCancel(ctx)
}

func (ei *resourceInformer) WithKubeClient(client klient.Client) {
	ei.KubeClient = client
}

func (ei *resourceInformer) WithMetricStorage(mstor *metric_storage.MetricStorage) {
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

	ei.ListOptions = metav1.ListOptions{
		FieldSelector: fmtFieldSelector,
		LabelSelector: fmtLabelSelector,
	}

	ei.FactoryIndex = FactoryIndex{
		GVR:           ei.GroupVersionResource,
		Namespace:     ei.Namespace,
		FieldSelector: ei.ListOptions.FieldSelector,
		LabelSelector: ei.ListOptions.LabelSelector,
	}

	err = ei.LoadExistedObjects()
	if err != nil {
		log.Errorf("load existing objects: %v", err)
		return err
	}

	return nil
}

// Snapshot returns all cached objects for this informer
func (ei *resourceInformer) CachedObjects() []ObjectAndFilterResult {
	ei.cacheLock.RLock()
	res := make([]ObjectAndFilterResult, 0)
	for _, obj := range ei.cachedObjects {
		res = append(res, *obj)
	}
	ei.cacheLock.RUnlock()

	// Reset eventBuf if needed.
	if !ei.eventCbEnabled {
		ei.eventBufLock.Lock()
		ei.eventBuf = nil
		ei.eventBufLock.Unlock()
	}
	return res
}

func (ei *resourceInformer) EnableKubeEventCb() {
	if ei.eventCbEnabled {
		return
	}
	ei.eventBufLock.Lock()
	defer ei.eventBufLock.Unlock()
	ei.eventCbEnabled = true
	for _, kubeEvent := range ei.eventBuf {
		// Handle saved kube events.
		ei.EventCb(kubeEvent)
	}
	ei.eventBuf = nil
}

// LoadExistedObjects get a list of existed objects in namespace that match selectors and
// fills Checksum map with checksums of existing objects.
func (ei *resourceInformer) LoadExistedObjects() error {
	defer trace.StartRegion(context.Background(), "LoadExistedObjects").End()

	objList, err := ei.KubeClient.Dynamic().
		Resource(ei.GroupVersionResource).
		Namespace(ei.Namespace).
		List(context.TODO(), ei.ListOptions)
	if err != nil {
		log.Errorf("%s: initial list resources of kind '%s': %v", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, err)
		return err
	}

	if objList == nil || len(objList.Items) == 0 {
		log.Debugf("%s: Got no existing '%s' resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind)
		return nil
	}

	// FIXME objList.Items has too much information for log
	// log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)
	log.Debugf("%s: '%s' initial list: Got %d existing resources", ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, len(objList.Items))

	filteredObjects := make(map[string]*ObjectAndFilterResult)

	for _, item := range objList.Items {
		// copy loop var to avoid duplication of pointer in filteredObjects
		obj := item

		var objFilterRes *ObjectAndFilterResult
		var err error
		func() {
			defer measure.Duration(func(d time.Duration) {
				ei.metricStorage.HistogramObserve("{PREFIX}kube_jq_filter_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
			})()
			objFilterRes, err = ApplyFilter(ei.Monitor.JqFilter, ei.Monitor.FilterFunc, &obj)
		}()

		if err != nil {
			return err
		}

		if !ei.Monitor.KeepFullObjectsInMemory {
			objFilterRes.RemoveFullObject()
		}

		filteredObjects[objFilterRes.Metadata.ResourceId] = objFilterRes

		log.Debugf("%s: initial list: '%s' is cached with checksum %s",
			ei.Monitor.Metadata.DebugName,
			objFilterRes.Metadata.ResourceId,
			objFilterRes.Metadata.Checksum)
	}

	// Save objects to the cache.
	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	for k, v := range filteredObjects {
		ei.cachedObjects[k] = v
	}

	ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
	ei.metricStorage.GaugeSet("{PREFIX}kube_snapshot_objects", float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)

	return nil
}

func (ei *resourceInformer) OnAdd(obj interface{}) {
	ei.HandleWatchEvent(nil, obj, WatchEventAdded)
}

func (ei *resourceInformer) OnUpdate(oldObj, newObj interface{}) {
	ei.HandleWatchEvent(oldObj, newObj, WatchEventModified)
}

func (ei *resourceInformer) OnDelete(obj interface{}) {
	ei.HandleWatchEvent(nil, obj, WatchEventDeleted)
}

// HandleKubeEvent register object in cache. Pass object to callback if object's checksum is changed.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modified is enough)
// func (ei *resourceInformer) HandleKubeEvent(obj *unstructured.Unstructured, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
func (ei *resourceInformer) HandleWatchEvent(oldObject, object interface{}, eventType WatchEventType) {
	// check if stop
	if ei.stopped {
		return
	}

	defer measure.Duration(func(d time.Duration) {
		ei.metricStorage.HistogramObserve("{PREFIX}kube_event_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
	})()
	defer trace.StartRegion(context.Background(), "HandleWatchEvent").End()

	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	obj := object.(*unstructured.Unstructured)

	var oldObj *unstructured.Unstructured
	if oldObject != nil {
	  oldObj = oldObject.(*unstructured.Unstructured)
  }

	resourceId := ResourceId(obj)

	// Always calculate checksum and update cache, because we need an actual state in ei.cachedObjects.

	var objFilterRes *ObjectAndFilterResult
	var err error
	func() {
		defer measure.Duration(func(d time.Duration) {
			ei.metricStorage.HistogramObserve("{PREFIX}kube_jq_filter_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
		})()
		objFilterRes, err = ApplyFilter(ei.Monitor.JqFilter, ei.Monitor.FilterFunc, obj)
	}()
	if err != nil {
		log.Errorf("%s: WATCH %s: %s",
			ei.Monitor.Metadata.DebugName,
			eventType,
			err)
		return
	}

	if !ei.Monitor.KeepFullObjectsInMemory {
		objFilterRes.RemoveFullObject()
	} else if ei.Monitor.KeepFullObjectsInMemory {
		if oldObj != nil {
			objFilterRes.OldObject = oldObj
		}
	}

	// Do not fire Added or Modified if object is in cache and its checksum is equal to the newChecksum.
	// Delete is always fired.
	switch eventType {
	case WatchEventAdded:
		fallthrough
	case WatchEventModified:
		// Update object in cache
		ei.cacheLock.Lock()
		cachedObject, objectInCache := ei.cachedObjects[resourceId]
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
		ei.cachedObjects[resourceId] = objFilterRes
		// Update cached objects info.
		ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
		if eventType == WatchEventAdded {
			ei.cachedObjectsInfo.Added++
			ei.cachedObjectsIncrement.Added++
		} else {
			ei.cachedObjectsInfo.Modified++
			ei.cachedObjectsIncrement.Modified++
		}
		// Update metrics.
		ei.metricStorage.GaugeSet("{PREFIX}kube_snapshot_objects", float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)
		ei.cacheLock.Unlock()
		if skipEvent {
			return
		}

	case WatchEventDeleted:
		ei.cacheLock.Lock()
		delete(ei.cachedObjects, resourceId)
		// Update cached objects info.
		ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
		if ei.cachedObjectsInfo.Count == 0 {
			ei.cachedObjectsInfo.Cleaned++
			ei.cachedObjectsIncrement.Cleaned++
		}
		ei.cachedObjectsInfo.Deleted++
		ei.cachedObjectsIncrement.Deleted++
		// Update metrics.
		ei.metricStorage.GaugeSet("{PREFIX}kube_snapshot_objects", float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)
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
		// log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)

		kubeEvent := KubeEvent{
			Type:        TypeEvent,
			MonitorId:   ei.Monitor.Metadata.MonitorId,
			WatchEvents: []WatchEventType{eventType},
			Objects:     []ObjectAndFilterResult{*objFilterRes},
		}

		// fix race with EnableKubeEventCb.
		eventCbEnabled := false
		ei.eventBufLock.Lock()
		eventCbEnabled = ei.eventCbEnabled
		ei.eventBufLock.Unlock()

		if eventCbEnabled {
			// Pass event info to callback.
			ei.EventCb(kubeEvent)
		} else {
			ei.eventBufLock.Lock()
			// Save event in buffer until the callback is enabled.
			if ei.eventBuf == nil {
				ei.eventBuf = make([]KubeEvent, 0)
			}
			ei.eventBuf = append(ei.eventBuf, kubeEvent)
			ei.eventBufLock.Unlock()
		}
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

	go func() {
		if ei.ctx != nil {
			<-ei.ctx.Done()
			DefaultFactoryStore.Stop(ei.FactoryIndex)
		}
	}()

	// TODO: separate handler and informer
	errorHandler := NewWatchErrorHandler(ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, ei.Monitor.Metadata.LogLabels, ei.metricStorage)
	err := DefaultFactoryStore.Start(ei.KubeClient.Dynamic(), ei.FactoryIndex, ei, errorHandler)
	if err != nil {
		ei.Monitor.LogEntry.Errorf("%s: cache is not synced for informer", ei.Monitor.Metadata.DebugName)
		return
	}

	log.Debugf("%s: informer is ready", ei.Monitor.Metadata.DebugName)
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

// CachedObjectsInfo returns info accumulated from start.
func (ei *resourceInformer) CachedObjectsInfo() CachedObjectsInfo {
	ei.cacheLock.RLock()
	defer ei.cacheLock.RUnlock()
	return *ei.cachedObjectsInfo
}

// CachedObjectsInfoIncrement returns info accumulated from last call and clean it.
func (ei *resourceInformer) CachedObjectsInfoIncrement() CachedObjectsInfo {
	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	info := *ei.cachedObjectsIncrement
	ei.cachedObjectsIncrement = &CachedObjectsInfo{}
	return info
}
