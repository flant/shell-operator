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

	"github.com/ldmonster/kubeclient/store"

	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/kube/dedupclient"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

type resourceInformer struct {
	id         string
	KubeClient *dedupclient.Client
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
	// When snapshotStore is non-nil, entries here have Object == nil and the
	// authoritative `*Unstructured` body lives once in the shared dedup store
	// keyed by cachedObjectKeys[resourceId]. getCachedObjects reconstitutes
	// Object lazily on read so the public type behaviour is unchanged.
	cachedObjects map[string]*kemtypes.ObjectAndFilterResult
	cacheLock     sync.RWMutex

	// snapshotStore, when non-nil, owns the storage of `*Unstructured`
	// bodies. Each resourceInformer is one *owner* (identified by its
	// id) so the store can refcount keys across overlapping watches.
	snapshotStore *dedupclient.SnapshotStore
	// cachedObjectKeys mirrors cachedObjects when snapshotStore is set.
	// It is kept under cacheLock together with cachedObjects.
	cachedObjectKeys map[string]store.ObjectKey

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
	client        *dedupclient.Client
	mstor         metricsstorage.Storage
	factoryStore  *FactoryStore
	snapshotStore *dedupclient.SnapshotStore
	eventCb       func(kemtypes.KubeEvent)
	monitor       *MonitorConfig

	logger *log.Logger
}

func newResourceInformer(ns, name string, cfg *resourceInformerConfig) *resourceInformer {
	informer := &resourceInformer{
		id:                     uuid.Must(uuid.NewV4()).String(),
		KubeClient:             cfg.client,
		metricStorage:          cfg.mstor,
		factoryStore:           cfg.factoryStore,
		snapshotStore:          cfg.snapshotStore,
		Namespace:              ns,
		Name:                   name,
		eventCb:                cfg.eventCb,
		Monitor:                cfg.monitor,
		cachedObjects:          make(map[string]*kemtypes.ObjectAndFilterResult),
		cachedObjectKeys:       make(map[string]store.ObjectKey),
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
		GVK: schema.GroupVersionKind{
			Group:   ei.GroupVersionResource.Group,
			Version: ei.GroupVersionResource.Version,
			Kind:    ei.Monitor.Kind,
		},
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
// When snapshotStore is set, every entry's `Object` field is reconstituted
// from the shared dedup store at call time. Reconstitution is a fresh
// allocation per call, so callers should treat the returned slice as
// owned and avoid pinning it longer than necessary. If the store has
// dropped a key concurrently (e.g. during shutdown) the returned entry
// keeps Object=nil; downstream code already tolerates that path because
// the same shape is used by Monitor.KeepFullObjectsInMemory==false.
func (ei *resourceInformer) getCachedObjects() []kemtypes.ObjectAndFilterResult {
	ei.cacheLock.RLock()
	res := make([]kemtypes.ObjectAndFilterResult, 0, len(ei.cachedObjects))
	if ei.snapshotStore != nil {
		for resID, entry := range ei.cachedObjects {
			cp := *entry
			if key, ok := ei.cachedObjectKeys[resID]; ok {
				if obj, found := ei.snapshotStore.Get(key); found {
					cp.Object = obj
				}
			}
			res = append(res, cp)
		}
	} else {
		for _, obj := range ei.cachedObjects {
			res = append(res, *obj)
		}
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

	filteredObjects := make(map[string]*kemtypes.ObjectAndFilterResult)
	// keysForStore is the set of (resourceId → ObjectKey) that need to be
	// committed to the shared snapshot store under cacheLock. We collect
	// them here so the dedup-store roundtrip happens before the lock is
	// taken, keeping the critical section small.
	var keysForStore map[string]store.ObjectKey
	if ei.snapshotStore != nil {
		keysForStore = make(map[string]store.ObjectKey, len(objList.Items))
	}

	for _, item := range objList.Items {
		// copy loop var to avoid duplication of pointer in filteredObjects
		obj := item

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

		if !ei.Monitor.KeepFullObjectsInMemory {
			objFilterRes.RemoveFullObject()
		} else if ei.snapshotStore != nil && objFilterRes.Object != nil {
			// Move the body into the shared dedup store and drop our
			// per-monitor pointer. RemoveObject MUST stay false so the
			// snapshot still surfaces the body once Get reconstitutes
			// it on read.
			key := dedupclient.KeyFor(objFilterRes.Object)
			if err := ei.snapshotStore.Acquire(ei.id, key, objFilterRes.Object); err != nil {
				ei.logger.Warn("snapshot store: acquire failed during initial list, falling back to local copy",
					slog.String(pkg.LogKeyResourceId, objFilterRes.Metadata.ResourceId),
					log.Err(err))
			} else {
				keysForStore[objFilterRes.Metadata.ResourceId] = key
				objFilterRes.Object = nil
			}
		}

		filteredObjects[objFilterRes.Metadata.ResourceId] = objFilterRes

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
	for k, key := range keysForStore {
		ei.cachedObjectKeys[k] = key
	}

	ei.cachedObjectsInfo.Count = uint64(len(ei.cachedObjects))
	ei.metricStorage.GaugeSet(metrics.KubeSnapshotObjects, float64(len(ei.cachedObjects)), ei.Monitor.Metadata.MetricLabels)

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
	obj := object.(*unstructured.Unstructured)

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

	// storeKey is the shared dedup-store key for this object. It is set
	// only when snapshotStore is enabled and KeepFullObjectsInMemory is
	// true (the path that materially benefits from deduplication). Other
	// paths leave storeKey at its zero value and skip Acquire/Release.
	var storeKey store.ObjectKey
	storeBacked := false

	if !ei.Monitor.KeepFullObjectsInMemory {
		objFilterRes.RemoveFullObject()
	} else if ei.snapshotStore != nil && objFilterRes.Object != nil {
		storeKey = dedupclient.KeyFor(objFilterRes.Object)
		if err := ei.snapshotStore.Acquire(ei.id, storeKey, objFilterRes.Object); err != nil {
			ei.logger.Warn("snapshot store: acquire failed on watch event, falling back to local copy",
				slog.String(pkg.LogKeyResourceId, resourceId),
				slog.String(pkg.LogKeyEventType, string(eventType)),
				log.Err(err))
		} else {
			storeBacked = true
			objFilterRes.Object = nil
		}
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
				slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName),
				slog.String(pkg.LogKeyEventType, string(eventType)),
				slog.String(pkg.LogKeyResourceId, resourceId),
			)
			skipEvent = true
		}
		ei.cachedObjects[resourceId] = objFilterRes
		if storeBacked {
			ei.cachedObjectKeys[resourceId] = storeKey
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
		// Drop the snapshot-store reference (if any) before forgetting
		// the resourceId — otherwise we leak ownership in the store.
		if ei.snapshotStore != nil {
			if key, ok := ei.cachedObjectKeys[resourceId]; ok {
				delete(ei.cachedObjectKeys, resourceId)
				// Release outside the lock would be cleaner, but the
				// store itself takes only its own (short-lived) mutex,
				// so contention here is bounded.
				if err := ei.snapshotStore.Release(ei.id, key); err != nil {
					ei.logger.Warn("snapshot store: release failed on delete event",
						slog.String(pkg.LogKeyResourceId, resourceId),
						log.Err(err))
				}
			}
		}
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
			// Drop every snapshot-store reference this informer still
			// holds. Without this the dedup store would leak entries
			// for stopped monitors until process exit.
			ei.releaseSnapshotStoreOwnership()
		}
	}()

	// TODO: separate handler and informer
	errorHandler := newWatchErrorHandler(ei.Monitor.Metadata.DebugName, ei.Monitor.Kind, ei.Monitor.Metadata.LogLabels, ei.metricStorage, ei.logger.Named("watch-error-handler"))
	err := ei.factoryStore.Start(ei.ctx, ei.id, ei.KubeClient.Dynamic(), ei.FactoryIndex, ei, errorHandler)
	if err != nil {
		ei.Monitor.Logger.Error("cache is not synced for informer", slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName))
		return
	}

	log.Debug("informer is ready", slog.String(pkg.LogKeyDebugName, ei.Monitor.Metadata.DebugName))
}

// releaseSnapshotStoreOwnership lets the shared dedup store reclaim every
// key this informer still owns. It is idempotent: calling it twice (or
// when snapshotStore is nil) is a safe no-op. Called from the start()
// shutdown goroutine so it runs on monitor cancellation.
func (ei *resourceInformer) releaseSnapshotStoreOwnership() {
	if ei.snapshotStore == nil {
		return
	}
	ei.snapshotStore.ReleaseOwner(ei.id)
	ei.cacheLock.Lock()
	// Clear the per-informer key map so subsequent reads see Object=nil.
	if len(ei.cachedObjectKeys) > 0 {
		ei.cachedObjectKeys = make(map[string]store.ObjectKey)
	}
	ei.cacheLock.Unlock()
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
