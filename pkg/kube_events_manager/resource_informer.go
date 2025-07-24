package kubeeventsmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/gofrs/uuid/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/filter"
	"github.com/flant/shell-operator/pkg/filter/jq"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

var resourceIDStore = NewResourceIDStore()

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

	// A cache for checksums to deduplicate events.
	checksumCache map[ResourceID]uint64
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
	// preallocate filter
	filter filter.Filter

	// TODO resourceInformer should be stoppable (think of deleted namespaces and disabled modules in addon-operator)
	ctx    context.Context
	cancel context.CancelFunc

	metricStorage metric.Storage

	// a flag to stop handle events after Stop()
	stopped atomic.Bool

	logger *log.Logger

	// objectPool is an "Object Transporter".
	// It's a pool of reusable unstructured.Unstructured objects that we use
	// to "teleport" data from the live cache into a safe, isolated container
	// before passing it to the JQ filter. This prevents race conditions
	// without the high memory/GC overhead of a simple DeepCopy on every call.
	objectPool *sync.Pool
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
		checksumCache:          make(map[ResourceID]uint64),
		cacheLock:              sync.RWMutex{},
		eventBufLock:           sync.Mutex{},
		cachedObjectsInfo:      &CachedObjectsInfo{},
		cachedObjectsIncrement: &CachedObjectsInfo{},
		logger:                 cfg.logger,
		filter:                 jq.NewFilter(),
		// Preallocate a pool of unstructured.Unstructured objects
		// to avoid allocations during the filter
		objectPool: &sync.Pool{
			New: func() interface{} {
				return &unstructured.Unstructured{}
			},
		},
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

	return nil
}

// populateChecksumCacheFromInformerStore fills the checksum cache from the informer's store.
func (ei *resourceInformer) populateChecksumCacheFromInformerStore() error {
	informer := DefaultFactoryStore.Get(ei.KubeClient.Dynamic(), ei.FactoryIndex).
		shared.ForResource(ei.GroupVersionResource).Informer()

	items := informer.GetStore().List()

	ei.cacheLock.Lock()
	defer ei.cacheLock.Unlock()
	ei.checksumCache = make(map[ResourceID]uint64, len(items))

	var allErrors error

	for i := range items {
		// don't store copy of the object in heap, just use pointer
		obj := items[i].(*unstructured.Unstructured)
		resourceID := resourceIDStore.GetResourceID(obj)

		// Using a closure to correctly manage the lifecycle of the pooled object with defer.
		func() {
			// Step 1: Get a safe, reusable object from the "Transporter" pool.
			pooledObj := ei.objectPool.Get().(*unstructured.Unstructured)
			defer ei.objectPool.Put(pooledObj)

			// Step 2: "Teleport" the data from the live object into our safe container.
			obj.DeepCopyInto(pooledObj)

			// Step 3: Use the safe object for filtering.
			// It's not a watch event, so we don't need to handle it.
			// We just need to fill the checksum cache.
			objFilterRes, err := ei.filterObject(obj)
			if err != nil {
				err = fmt.Errorf("cannot apply filter to initial object %s: %w", resourceID, err)
				allErrors = errors.Join(allErrors, err)
				log.Error("cannot apply filter to initial object",
					slog.String("debugName", ei.Monitor.Metadata.DebugName),
					slog.String("resourceId", resourceID.String()),
					log.Err(err))
				return
			}
			// calculate checksum for filtered data
			checksum, err := checksum.CalculateChecksum(objFilterRes.FilterResult)
			if err != nil {
				err = fmt.Errorf("cannot calculate checksum for initial object %s: %w", resourceID, err)
				allErrors = errors.Join(allErrors, err)
				log.Error("cannot calculate checksum for initial object",
					slog.String("debugName", ei.Monitor.Metadata.DebugName),
					slog.String("resourceId", resourceID.String()),
					log.Err(err))
				return
			}
			ei.checksumCache[resourceID] = checksum
		}()
	}
	log.Debug("informer: checksum cache populated", slog.String("informer", ei.Monitor.Metadata.DebugName), slog.Int("items", len(ei.checksumCache)))
	return allErrors
}

// filterObject is a central method to safely apply JQ filters to an object.
// It uses a sync.Pool to avoid memory allocations, prevent race conditions, and measures execution time.
func (ei *resourceInformer) filterObject(obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	var objFilterRes *kemtypes.ObjectAndFilterResult
	var err error

	// Using a closure to correctly manage the lifecycle of the pooled object with defer
	// and to measure the duration of the filtering process.
	func() {
		defer measure.Duration(func(d time.Duration) {
			ei.metricStorage.HistogramObserve("{PREFIX}kube_jq_filter_duration_seconds", d.Seconds(), ei.Monitor.Metadata.MetricLabels, nil)
		})()

		pooledObj := ei.objectPool.Get().(*unstructured.Unstructured)
		defer ei.objectPool.Put(pooledObj)

		obj.DeepCopyInto(pooledObj)

		objFilterRes, err = applyFilter(ei.Monitor.JqFilter, ei.filter, ei.Monitor.FilterFunc, pooledObj)
	}()
	if err != nil {
		return nil, err
	}
	objFilterRes.Object = obj
	return objFilterRes, nil
}

func (ei *resourceInformer) processSnapshotObject(obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	resourceID := resourceIDStore.GetResourceID(obj)

	// All the pooling and filtering logic is now in filterObject.
	objFilterRes, err := ei.filterObject(obj)
	if err != nil {
		return nil, fmt.Errorf("filter snapshot object: %w", err)
	}

	// If a filter is specified and it returns an empty result,
	// this object should not be included in the snapshot for the hook.
	if (ei.Monitor.JqFilter != nil || ei.Monitor.FilterFunc != nil) && (objFilterRes.FilterResult == nil || objFilterRes.FilterResult == "") {
		return nil, nil // Not an error, just skip this object.
	}

	// Use the already calculated and cached checksum.
	// This is the core optimization to prevent CPU spikes.
	if cachedChecksum, ok := ei.checksumCache[resourceID]; ok {
		objFilterRes.Metadata.Checksum = cachedChecksum
	} else {
		// As a fallback, calculate it on the fly from the original object.
		fallbackChecksum, errCalc := checksum.CalculateChecksum(obj)
		if errCalc != nil {
			return nil, fmt.Errorf("calculate checksum for snapshot object: %w", errCalc)
		}
		objFilterRes.Metadata.Checksum = fallbackChecksum
	}

	return objFilterRes, nil
}

// getCachedObjects returns a snapshot of objects from the informer's store.
func (ei *resourceInformer) getCachedObjects() []kemtypes.ObjectAndFilterResult {
	informer := DefaultFactoryStore.Get(ei.KubeClient.Dynamic(), ei.FactoryIndex).
		shared.ForResource(ei.GroupVersionResource).Informer()

	items := informer.GetStore().List()
	res := make([]kemtypes.ObjectAndFilterResult, 0, len(items))

	ei.cacheLock.RLock()
	defer ei.cacheLock.RUnlock()

	for i := range items {
		obj := items[i].(*unstructured.Unstructured)

		objFilterRes, err := ei.processSnapshotObject(obj)
		if err != nil {
			log.Error("cannot process snapshot object",
				slog.String("debugName", ei.Monitor.Metadata.DebugName),
				slog.String("resourceId", resourceIDStore.GetResourceID(obj).String()),
				log.Err(err))
			continue
		}

		if objFilterRes != nil {
			res = append(res, *objFilterRes)
		}
	}

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
	if ei.stopped.Load() {
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

	// The filterObject method now handles pooling, copying, filtering and metrics.
	objFilterRes, err := ei.filterObject(obj)
	if err != nil {
		log.Error("cannot apply filter to watch object",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("eventType", string(eventType)),
			log.Err(err))
		return
	}

	newChecksum, err := checksum.CalculateChecksum(objFilterRes.FilterResult)
	if err != nil {
		log.Error("handleWatchEvent: calculate checksum error",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("eventType", string(eventType)),
			log.Err(err))
		return
	}
	objFilterRes.Metadata.Checksum = newChecksum

	if !ei.Monitor.KeepFullObjectsInMemory {
		objFilterRes.RemoveFullObject()
	}

	// Do not fire Added or Modified if object is in cache and its checksum is equal to the newChecksum.
	// Delete is always fired.
	switch eventType {
	case kemtypes.WatchEventAdded:
		fallthrough
	case kemtypes.WatchEventModified:
		ei.cacheLock.Lock()
		oldChecksum := ei.checksumCache[resourceIDStore.GetResourceID(obj)]
		ei.checksumCache[resourceIDStore.GetResourceID(obj)] = newChecksum
		ei.cacheLock.Unlock()

		if oldChecksum == newChecksum {
			return
		}

	case kemtypes.WatchEventDeleted:
		ei.cacheLock.Lock()
		delete(ei.checksumCache, resourceIDStore.GetResourceID(obj))
		ei.cacheLock.Unlock()
	}

	// Fire KubeEvent only if needed.
	if ei.shouldFireEvent(eventType) {
		log.Debug("send KubeEvent",
			slog.String("debugName", ei.Monitor.Metadata.DebugName),
			slog.String("eventType", string(eventType)),
			slog.String("resourceId", objFilterRes.Metadata.ResourceId))
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

	// Populate the checksum cache for the first time.
	err = ei.populateChecksumCacheFromInformerStore()
	if err != nil {
		ei.Monitor.Logger.Error("cannot populate checksum cache", log.Err(err))
	}

	log.Debug("informer is ready", slog.String("debugName", ei.Monitor.Metadata.DebugName))
}

func (ei *resourceInformer) pauseHandleEvents() {
	log.Debug("PAUSE resource informer", slog.String("debugName", ei.Monitor.Metadata.DebugName))
	ei.stopped.Store(true)
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
