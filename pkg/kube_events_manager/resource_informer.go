package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/trace"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/gofrs/uuid/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	klient "github.com/flant/kube-client/client"
	pkg "github.com/flant/shell-operator/pkg"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

// cachedEntry is the slim per-object cache value held by resourceInformer.
// The full *unstructured.Unstructured is intentionally NOT stored here: it
// lives in the SharedIndexInformer's indexer (the same client-go cache shared
// across the operator). When a snapshot is materialised, the live object is
// resolved on demand via FactoryStore.GetByKey, which removes the per-binding
// duplicate map of object pointers and the per-entry ObjectAndFilterResult
// wrapper (Metadata.JqFilter / ResourceId / RemoveObject).
type cachedEntry struct {
	// IndexerKey is the SharedIndexInformer cache key (namespace/name or name
	// for cluster-scoped resources). Used to resolve the live object.
	IndexerKey string
	// Checksum of the filter result; compared on update to suppress no-op events.
	Checksum string
	// FilterResult is the (typically small) jq output or filter-func result
	// that hooks consume directly.
	FilterResult any
}

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

	// A cache of (slim) filter results keyed by resourceId. It is a part of the
	// Monitor's snapshot. The live *unstructured.Unstructured for each entry is
	// looked up from the SharedIndexInformer's indexer when the snapshot is
	// materialised, so this map does not hold a second copy of the objects.
	cachedObjects map[string]*cachedEntry
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

	metricStorage metricsstorage.Storage

	factoryStore *FactoryStore

	// a flag to stop handle events after Stop()
	stopped bool

	logger *log.Logger
}

// resourceInformer should implement ResourceInformer
type resourceInformerConfig struct {
	client       *klient.Client
	mstor        metricsstorage.Storage
	factoryStore *FactoryStore
	eventCb      func(kemtypes.KubeEvent)
	monitor      *MonitorConfig

	logger *log.Logger
}

func newResourceInformer(ns, name string, cfg *resourceInformerConfig) *resourceInformer {
	informer := &resourceInformer{
		id:                     uuid.Must(uuid.NewV4()).String(),
		KubeClient:             cfg.client,
		metricStorage:          cfg.mstor,
		factoryStore:           cfg.factoryStore,
		Namespace:              ns,
		Name:                   name,
		eventCb:                cfg.eventCb,
		Monitor:                cfg.monitor,
		cachedObjects:          make(map[string]*cachedEntry),
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
		slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
		slog.String(pkg.LogKeyAPIVersion, ei.Monitor.ApiVersion),
		slog.String(pkg.LogKeyKind, ei.Monitor.Kind))
	if ei.GroupVersionResource, err = ei.KubeClient.GroupVersionResource(ei.Monitor.ApiVersion, ei.Monitor.Kind); err != nil {
		return fmt.Errorf("%s: Cannot get GroupVersionResource info for apiVersion '%s' kind '%s' from api-server. Possibly CRD is not created before informers are started. Error was: %w", ei.Monitor.Metadata.DebugName, ei.Monitor.ApiVersion, ei.Monitor.Kind, err)
	}
	log.Debug("GVR for kind",
		slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
		slog.String(pkg.LogKeyKind, ei.Monitor.Kind),
		slog.String(pkg.LogKeyGVR, ei.GroupVersionResource.String()))

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

// Snapshot returns all cached objects for this informer.
//
// The slim cache stores only the filter result and checksum per object;
// the full *unstructured.Unstructured is resolved here from the
// SharedIndexInformer's indexer (a shared cache that already holds every
// monitored object). This avoids a second per-binding map of object pointers.
func (ei *resourceInformer) getCachedObjects() []kemtypes.ObjectAndFilterResult {
	ei.cacheLock.RLock()
	keepObjects := ei.Monitor.KeepFullObjectsInMemory
	jqFilter := ei.Monitor.JqFilter
	res := make([]kemtypes.ObjectAndFilterResult, 0, len(ei.cachedObjects))
	for resourceId, entry := range ei.cachedObjects {
		ofr := kemtypes.ObjectAndFilterResult{
			FilterResult: entry.FilterResult,
		}
		ofr.Metadata.JqFilter = jqFilter
		ofr.Metadata.Checksum = entry.Checksum
		ofr.Metadata.ResourceId = resourceId
		ofr.Metadata.RemoveObject = !keepObjects
		if keepObjects {
			ofr.Object, _ = ei.factoryStore.GetByKey(ei.FactoryIndex, entry.IndexerKey)
		}
		res = append(res, ofr)
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
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyKind, ei.Monitor.Kind),
			log.Err(err))
		return err
	}

	if objList == nil || len(objList.Items) == 0 {
		log.Debug("Got no existing resources",
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyKind, ei.Monitor.Kind))
		return nil
	}

	// FIXME objList.Items has too much information for log
	// log.Debugf("%s: Got %d existing '%s' resources: %+v", ei.Monitor.Metadata.DebugName, len(objList.Items), ei.Monitor.Kind, objList.Items)
	log.Debug("initial list: Got existing resources",
		slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
		slog.String(pkg.LogKeyKind, ei.Monitor.Kind),
		slog.Int(pkg.LogKeyCount, len(objList.Items)))

	filteredObjects := make(map[string]*cachedEntry, len(objList.Items))

	for _, item := range objList.Items {
		// copy loop var; needed because applyFilter takes a pointer.
		obj := item
		// The dynamic client's List() bypasses the SharedIndexInformer transform,
		// so apply the same field stripping here to keep cached objects small.
		stripUnstructuredNoise(&obj)

		var objFilterRes *kemtypes.ObjectAndFilterResult
		var err error
		func() {
			defer measure.Duration(func(d time.Duration) {
				ei.metricStorage.HistogramObserve(metrics.KubeJqFilterDurationSeconds, d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
			})()
			objFilterRes, err = applyFilter(ei.Monitor.CompiledJqFilter, ei.Monitor.JqFilter, ei.Monitor.FilterFunc, &obj)
		}()

		if err != nil {
			return fmt.Errorf("apply filter for '%s/%s': %w", ei.Monitor.Metadata.DebugName, obj.GetName(), err)
		}

		filteredObjects[objFilterRes.Metadata.ResourceId] = &cachedEntry{
			IndexerKey:   indexerKey(&obj),
			Checksum:     objFilterRes.Metadata.Checksum,
			FilterResult: objFilterRes.FilterResult,
		}

		log.Debug("initial list: cached with checksum",
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyResourceId, objFilterRes.Metadata.ResourceId),
			slog.String(pkg.LogKeyChecksum, objFilterRes.Metadata.Checksum))
	}

	// Save objects to the cache.
	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	for k, v := range filteredObjects {
		ei.cachedObjects[k] = v
	}

	ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
	ei.metricStorage.GaugeSet(metrics.KubeSnapshotObjects, float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)

	return nil
}

// indexerKey returns the cache.MetaNamespaceKeyFunc-style key for the given
// object: "namespace/name" for namespaced resources, "name" for cluster-scoped.
func indexerKey(obj *unstructured.Unstructured) string {
	ns := obj.GetNamespace()
	if ns == "" {
		return obj.GetName()
	}
	return ns + "/" + obj.GetName()
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
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyNamespace, ei.Namespace),
			slog.String(pkg.LogKeyName, ei.Name),
			slog.String(pkg.LogKeyEventType, string(eventType)))
		return
	}

	defer measure.Duration(func(d time.Duration) {
		ei.metricStorage.HistogramObserve(metrics.KubeEventDurationSeconds, d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
	})()
	defer trace.StartRegion(context.Background(), "handleWatchEvent").End()

	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	// The cache may hold either *unstructured.Unstructured (default) or a
	// typed runtime.Object (when typed-informer support is enabled and the
	// GVR is registered). Normalise to *unstructured.Unstructured here so the
	// rest of the pipeline keeps working unchanged.
	obj := toUnstructured(object)
	if obj == nil {
		log.Error("handleWatchEvent: could not normalise object to unstructured",
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyEventType, string(eventType)))
		return
	}
	// Idempotent: SetTransform on the SharedIndexInformer already strips noise
	// for objects entering the cache, but stripping again here is cheap and
	// guards against any code path that bypasses the transform (and against
	// objects rebuilt from the typed cache).
	stripUnstructuredNoise(obj)

	resourceId := resourceId(obj)

	// Always calculate checksum and update cache, because we need an actual state in ei.cachedObjects.

	var objFilterRes *kemtypes.ObjectAndFilterResult
	var err error
	func() {
		defer measure.Duration(func(d time.Duration) {
			ei.metricStorage.HistogramObserve(metrics.KubeJqFilterDurationSeconds, d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
		})()
		objFilterRes, err = applyFilter(ei.Monitor.CompiledJqFilter, ei.Monitor.JqFilter, ei.Monitor.FilterFunc, obj)
	}()
	if err != nil {
		log.Error("handleWatchEvent: applyFilter error",
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyEventType, string(eventType)),
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
		if objectInCache && cachedObject.Checksum == objFilterRes.Metadata.Checksum {
			// update object in cache and do not send event
			log.Debug("skip KubeEvent",
				slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
				slog.String(pkg.LogKeyEventType, string(eventType)),
				slog.String(pkg.LogKeyResourceId, resourceId),
			)
			skipEvent = true
		}
		ei.cachedObjects[resourceId] = &cachedEntry{
			IndexerKey:   indexerKey(obj),
			Checksum:     objFilterRes.Metadata.Checksum,
			FilterResult: objFilterRes.FilterResult,
		}
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
		ei.metricStorage.GaugeSet(metrics.KubeSnapshotObjects, float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)
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
		ei.metricStorage.GaugeSet(metrics.KubeSnapshotObjects, float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)
		ei.cacheLock.Unlock()
	}

	// Fire KubeEvent only if needed.
	if ei.shouldFireEvent(eventType) {
		log.Debug("send KubeEvent",
			slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
			slog.String(pkg.LogKeyEventType, string(eventType)),
			slog.String(pkg.LogKeyResourceId, resourceId))
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
	log.Debug("RUN resource informer", slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName))

	go func() {
		if ei.ctx != nil {
			<-ei.ctx.Done()
			ei.factoryStore.Stop(ei.id, ei.FactoryIndex)
		}
	}()

	// TODO: separate handler and informer
	errorHandler := newWatchErrorHandler(ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, ei.Monitor.Metadata.LogLabels, ei.metricStorage, ei.logger.Named("watch-error-handler"))
	// ei.KubeClient is *klient.Client, which embeds kubernetes.Interface and
	// therefore satisfies it. Passing it directly lets the FactoryStore
	// materialise a typed SharedInformerFactory when the typed-informer code
	// path is enabled and the GVR is registered.
	err := ei.factoryStore.Start(ei.ctx, ei.id, ei.KubeClient.Dynamic(), ei.KubeClient, ei.FactoryIndex, ei, errorHandler)
	if err != nil {
		ei.Monitor.Logger.Error("cache is not synced for informer", slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName))
		return
	}

	log.Debug("informer is ready", slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName))
}

// wait blocks until the underlying shared informer for this FactoryIndex is stopped
func (ei *resourceInformer) wait() {
	ei.factoryStore.WaitStopped(ei.FactoryIndex)
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
