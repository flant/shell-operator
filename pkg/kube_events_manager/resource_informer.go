package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/trace"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/gofrs/uuid/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/filter/jq"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

type resourceInformer struct {
	id         string
	KubeClient *klient.Client
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
	cachedObjects map[string]*kemtypes.ObjectAndFilterResult
	cacheLock     sync.RWMutex

	// Cached objects operations since start
	cachedObjectsInfo *CachedObjectsInfo
	// Cached objects operations since last access
	cachedObjectsIncrement *CachedObjectsInfo

	// Events buffer for "Synchronization" mode: it stores events between CachedObjects call and enableKubeEventCb call
	// to replay them when "Synchronization" hook is done.
	eventBuf []kemtypes.KubeEvent
	// A callback function that define custom handling of Kubernetes events.
	eventCb        func(kemtypes.KubeEvent)
	eventCbEnabled bool
	eventBufLock   sync.Mutex

	// TODO resourceInformer should be stoppable (think of deleted namespaces and disabled modules in addon-operator)
	ctx    context.Context
	cancel context.CancelFunc

	metricStorage metric.Storage

	// a flag to stop handle events after Stop()
	stopped bool

	logger *log.Logger
}

// resourceInformer should implement ResourceInformer
type resourceInformerConfig struct {
	client  *klient.Client
	mstor   metric.Storage
	eventCb func(kemtypes.KubeEvent)
	monitor *MonitorConfig

	logger *log.Logger
}

func newResourceInformer(ns, name string, cfg *resourceInformerConfig) *resourceInformer {
	informer := &resourceInformer{
		id:                     uuid.Must(uuid.NewV4()).String(),
		KubeClient:             cfg.client,
		metricStorage:          cfg.mstor,
		Namespace:              ns,
		Name:                   name,
		eventCb:                cfg.eventCb,
		Monitor:                cfg.monitor,
		cachedObjects:          make(map[string]*kemtypes.ObjectAndFilterResult),
		cacheLock:              sync.RWMutex{},
		eventBufLock:           sync.Mutex{},
		cachedObjectsInfo:      &CachedObjectsInfo{},
		cachedObjectsIncrement: &CachedObjectsInfo{},
		logger:                 cfg.logger,
	}
	return informer
}

func (ei *resourceInformer) withContext(ctx context.Context) {
	ei.ctx, ei.cancel = context.WithCancel(ctx)
}

func (ei *resourceInformer) putEvent(ev kemtypes.KubeEvent) {
	if ei.eventCb != nil {
		ei.eventCb(ev)
	}
}

func (ei *resourceInformer) createSharedInformer() error {
	var err error

	// discover GroupVersionResource for informer
	log.Debug("discover GVR for apiVersion...",
		slog.String("debugName", ei.Monitor.Metadata.DebugName),
		slog.String("apiVersion", ei.Monitor.ApiVersion),
		slog.String("kind", ei.Monitor.Kind))
	if ei.GroupVersionResource, err = ei.KubeClient.GroupVersionResource(ei.Monitor.ApiVersion, ei.Monitor.Kind); err != nil {
		return fmt.Errorf("%s: Cannot get GroupVersionResource info for apiVersion '%s' kind '%s' from api-server. Possibly CRD is not created before informers are started. Error was: %w", ei.Monitor.Metadata.DebugName, ei.Monitor.ApiVersion, ei.Monitor.Kind, err)
	}
	log.Debug("GVR for kind",
		slog.String("debugName", ei.Monitor.Metadata.DebugName),
		slog.String("kind", ei.Monitor.Kind),
		slog.String("gvr", ei.GroupVersionResource.String()))

	// define tweakListOptions for informer
	fmtLabelSelector, err := FormatLabelSelector(ei.Monitor.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %w", ei.Monitor.LabelSelector.String(), err)
	}

	fieldSelector := ei.adjustFieldSelector(ei.Monitor.FieldSelector, ei.Name)
	fmtFieldSelector, err := FormatFieldSelector(fieldSelector)
	if err != nil {
		return fmt.Errorf("format field selector '%+v': %w", fieldSelector, err)
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

	if err = ei.loadExistedObjects(); err != nil {
		return fmt.Errorf("load existing objects: %w", err)
	}

	return nil
}

// Snapshot returns all cached objects for this informer
func (ei *resourceInformer) getCachedObjects() []kemtypes.ObjectAndFilterResult {
	ei.cacheLock.RLock()
	res := make([]kemtypes.ObjectAndFilterResult, 0)
	for _, obj := range ei.cachedObjects {
		res = append(res, *obj)
	}
	ei.cacheLock.RUnlock()

	// Reset eventBuf if needed.
	ei.eventBufLock.Lock()
	if !ei.eventCbEnabled {
		ei.eventBuf = nil
	}
	ei.eventBufLock.Unlock()
	return res
}

func (ei *resourceInformer) enableKubeEventCb() {
	ei.eventBufLock.Lock()
	defer ei.eventBufLock.Unlock()
	if ei.eventCbEnabled {
		return
	}
	ei.eventCbEnabled = true
	for _, kubeEvent := range ei.eventBuf {
		// Handle saved kube events.
		ei.putEvent(kubeEvent)
	}
	ei.eventBuf = nil
}

// loadExistedObjects get a list of existed objects in namespace that match selectors and
// fills Checksum map with checksums of existing objects.
func (ei *resourceInformer) loadExistedObjects() error {
	defer trace.StartRegion(context.Background(), "loadExistedObjects").End()

	objList, err := ei.KubeClient.Dynamic().
		Resource(ei.GroupVersionResource).
		Namespace(ei.Namespace).
		List(context.TODO(), ei.ListOptions)
	if err != nil {
		log.Error("initial list resources of kind",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("kind", ei.Monitor.Kind),
			log.Err(err))
		return err
	}

	if objList == nil || len(objList.Items) == 0 {
		log.Debug("Got no existing resources",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("kind", ei.Monitor.Kind))
		return nil
	}

	// FIXME objList.Items has too much information for log
	// log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)
	log.Debug("initial list: Got existing resources",
		slog.String("debugName", ei.Monitor.Metadata.DebugName),
		slog.String("kind", ei.Monitor.Kind),
		slog.Int("count", len(objList.Items)))

	filteredObjects := make(map[string]*kemtypes.ObjectAndFilterResult)

	for _, item := range objList.Items {
		// copy loop var to avoid duplication of pointer in filteredObjects
		obj := item

		var objFilterRes *kemtypes.ObjectAndFilterResult
		var err error
		func() {
			defer measure.Duration(func(d time.Duration) {
				ei.metricStorage.HistogramObserve("{PREFIX}kube_jq_filter_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
			})()
			filter := jq.NewFilter(app.JqLibraryPath)
			objFilterRes, err = applyFilter(ei.Monitor.JqFilter, filter, ei.Monitor.FilterFunc, &obj)
		}()

		if err != nil {
			return err
		}

		if !ei.Monitor.KeepFullObjectsInMemory {
			objFilterRes.RemoveFullObject()
		}

		filteredObjects[objFilterRes.Metadata.ResourceId] = objFilterRes

		log.Debug("initial list: cached with checksum",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("resourceId", objFilterRes.Metadata.ResourceId),
			slog.String("checksum", objFilterRes.Metadata.Checksum))
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

func (ei *resourceInformer) OnAdd(obj interface{}, _ bool) {
	ei.handleWatchEvent(obj, kemtypes.WatchEventAdded)
}

func (ei *resourceInformer) OnUpdate(_, newObj interface{}) {
	ei.handleWatchEvent(newObj, kemtypes.WatchEventModified)
}

func (ei *resourceInformer) OnDelete(obj interface{}) {
	ei.handleWatchEvent(obj, kemtypes.WatchEventDeleted)
}

// HandleKubeEvent register object in cache. Pass object to callback if object's checksum is changed.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modified is enough)
// func (ei *resourceInformer) HandleKubeEvent(obj *unstructured.Unstructured, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
func (ei *resourceInformer) handleWatchEvent(object interface{}, eventType kemtypes.WatchEventType) {
	// check if stop
	if ei.stopped {
		log.Debug("received WATCH for stopped informer",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("namespace", ei.Namespace),
			slog.String("name", ei.Name),
			slog.String("eventType", string(eventType)))
		return
	}

	defer measure.Duration(func(d time.Duration) {
		ei.metricStorage.HistogramObserve("{PREFIX}kube_event_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
	})()
	defer trace.StartRegion(context.Background(), "handleWatchEvent").End()

	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	obj := object.(*unstructured.Unstructured)

	resourceId := resourceId(obj)

	// Always calculate checksum and update cache, because we need an actual state in ei.cachedObjects.

	var objFilterRes *kemtypes.ObjectAndFilterResult
	var err error
	func() {
		defer measure.Duration(func(d time.Duration) {
			ei.metricStorage.HistogramObserve("{PREFIX}kube_jq_filter_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
		})()
		filter := jq.NewFilter(app.JqLibraryPath)
		objFilterRes, err = applyFilter(ei.Monitor.JqFilter, filter, ei.Monitor.FilterFunc, obj)
	}()
	if err != nil {
		log.Error("handleWatchEvent: applyFilter error",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("eventType", string(eventType)),
			log.Err(err))
		return
	}

	if !ei.Monitor.KeepFullObjectsInMemory {
		objFilterRes.RemoveFullObject()
	}

	// Do not fire Added or Modified if object is in cache and its checksum is equal to the newChecksum.
	// Delete is always fired.
	switch eventType {
	case kemtypes.WatchEventAdded:
		fallthrough
	case kemtypes.WatchEventModified:
		// Update object in cache
		ei.cacheLock.Lock()
		cachedObject, objectInCache := ei.cachedObjects[resourceId]
		skipEvent := false
		if objectInCache && cachedObject.Metadata.Checksum == objFilterRes.Metadata.Checksum {
			// update object in cache and do not send event
			log.Debug("skip KubeEvent",
				slog.String("debugName", ei.Monitor.Metadata.DebugName),
				slog.String("eventType", string(eventType)),
				slog.String("resourceId", resourceId),
			)
			skipEvent = true
		}
		ei.cachedObjects[resourceId] = objFilterRes
		// Update cached objects info.
		ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
		if eventType == kemtypes.WatchEventAdded {
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

	case kemtypes.WatchEventDeleted:
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
	if ei.shouldFireEvent(eventType) {
		log.Debug("send KubeEvent",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("eventType", string(eventType)),
			slog.String("resourceId", resourceId))
		// TODO: should be disabled by default and enabled by a debug feature switch
		// log.Debugf("HandleKubeEvent: obj type is %T, value:\n%#v", obj, obj)

		kubeEvent := kemtypes.KubeEvent{
			Type:        kemtypes.TypeEvent,
			MonitorId:   ei.Monitor.Metadata.MonitorId,
			WatchEvents: []kemtypes.WatchEventType{eventType},
			Objects:     []kemtypes.ObjectAndFilterResult{*objFilterRes},
		}

		// fix race with enableKubeEventCb.
		eventCbEnabled := false
		ei.eventBufLock.Lock()
		eventCbEnabled = ei.eventCbEnabled
		ei.eventBufLock.Unlock()

		if eventCbEnabled {
			// Pass event info to callback.
			ei.putEvent(kubeEvent)
		} else {
			ei.eventBufLock.Lock()
			// Save event in buffer until the callback is enabled.
			if ei.eventBuf == nil {
				ei.eventBuf = make([]kemtypes.KubeEvent, 0)
			}
			ei.eventBuf = append(ei.eventBuf, kubeEvent)
			ei.eventBufLock.Unlock()
		}
	}
}

func (ei *resourceInformer) adjustFieldSelector(selector *kemtypes.FieldSelector, objName string) *kemtypes.FieldSelector {
	var selectorCopy *kemtypes.FieldSelector

	if selector != nil {
		selectorCopy = &kemtypes.FieldSelector{
			MatchExpressions: selector.MatchExpressions,
		}
	}

	if objName != "" {
		objNameReq := kemtypes.FieldSelectorRequirement{
			Field:    "metadata.name",
			Operator: "=",
			Value:    objName,
		}
		if selectorCopy == nil {
			selectorCopy = &kemtypes.FieldSelector{
				MatchExpressions: []kemtypes.FieldSelectorRequirement{
					objNameReq,
				},
			}
		} else {
			selectorCopy.MatchExpressions = append(selectorCopy.MatchExpressions, objNameReq)
		}
	}

	return selectorCopy
}

func (ei *resourceInformer) shouldFireEvent(checkEvent kemtypes.WatchEventType) bool {
	for _, event := range ei.Monitor.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *resourceInformer) start() {
	log.Debug("RUN resource informer", slog.String("debugName", ei.Monitor.Metadata.DebugName))

	go func() {
		if ei.ctx != nil {
			<-ei.ctx.Done()
			DefaultFactoryStore.Stop(ei.id, ei.FactoryIndex)
		}
	}()

	// TODO: separate handler and informer
	errorHandler := newWatchErrorHandler(ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, ei.Monitor.Metadata.LogLabels, ei.metricStorage, ei.logger.Named("watch-error-handler"))
	err := DefaultFactoryStore.Start(ei.ctx, ei.id, ei.KubeClient.Dynamic(), ei.FactoryIndex, ei, errorHandler)
	if err != nil {
		ei.Monitor.Logger.Error("cache is not synced for informer", slog.String("debugName", ei.Monitor.Metadata.DebugName))
		return
	}

	log.Debug("informer is ready", slog.String("debugName", ei.Monitor.Metadata.DebugName))
}

func (ei *resourceInformer) pauseHandleEvents() {
	log.Debug("PAUSE resource informer", slog.String("debugName", ei.Monitor.Metadata.DebugName))
	ei.stopped = true
}

// CachedObjectsInfo returns info accumulated from start.
func (ei *resourceInformer) getCachedObjectsInfo() CachedObjectsInfo {
	ei.cacheLock.RLock()
	defer ei.cacheLock.RUnlock()
	return *ei.cachedObjectsInfo
}

// getCachedObjectsInfoIncrement returns info accumulated from last call and clean it.
func (ei *resourceInformer) getCachedObjectsInfoIncrement() CachedObjectsInfo {
	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	info := *ei.cachedObjectsIncrement
	ei.cachedObjectsIncrement = &CachedObjectsInfo{}
	return info
}
