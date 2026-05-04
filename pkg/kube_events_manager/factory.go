package kubeeventsmanager

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	pkg "github.com/flant/shell-operator/pkg"
)

const (
	FactoryShutdownTimeout = 30 * time.Second
)

var DefaultSyncTime = 100 * time.Millisecond

type FactoryIndex struct {
	GVR           schema.GroupVersionResource
	Namespace     string
	FieldSelector string
	LabelSelector string
}

// Factory tracks one SharedIndexInformer plus the bookkeeping needed to
// share it across multiple monitor handlers. The underlying informer may be
// produced by either client-go's dynamicinformer package (default) or by the
// typed informers package (when typed-informer support is enabled and the
// GVR is registered). Both produce a cache.SharedIndexInformer, so consumers
// of Factory only need to know about that single interface.
type Factory struct {
	informer             cache.SharedIndexInformer
	handlerRegistrations map[string]cache.ResourceEventHandlerRegistration
	ctx                  context.Context
	cancel               context.CancelFunc
	// typed reports whether the informer's cache stores typed runtime.Object
	// values (e.g. *corev1.Pod) instead of *unstructured.Unstructured. Used to
	// pick the right TransformFunc and to inform read paths.
	typed bool
	// done is closed when the underlying informer.Run returns
	done chan struct{}
}

type FactoryStore struct {
	mu        sync.Mutex
	data      map[FactoryIndex]*Factory
	stoppedCh map[FactoryIndex]chan struct{}
}

func NewFactoryStore() *FactoryStore {
	fs := &FactoryStore{
		data:      make(map[FactoryIndex]*Factory),
		stoppedCh: make(map[FactoryIndex]chan struct{}),
	}
	return fs
}

func (c *FactoryStore) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[FactoryIndex]*Factory)
	c.stoppedCh = make(map[FactoryIndex]chan struct{})
}

func (c *FactoryStore) add(ctx context.Context, index FactoryIndex, informer cache.SharedIndexInformer, typed bool) {
	ctx, cancel := context.WithCancel(ctx)
	c.data[index] = &Factory{
		informer:             informer,
		handlerRegistrations: make(map[string]cache.ResourceEventHandlerRegistration),
		ctx:                  ctx,
		cancel:               cancel,
		typed:                typed,
		done:                 nil,
	}

	log.Debug("Factory store: added a new factory for index",
		slog.String(pkg.LogKeyNamespace, index.Namespace),
		slog.String(pkg.LogKeyGVR, index.GVR.String()),
		slog.Bool("typed", typed))
}

// newInformerForIndex builds a SharedIndexInformer for the given index. When
// the typed-informer code path is enabled and the GVR is registered with a
// typed converter, this returns a typed informer materialised by client-go's
// informers.SharedInformerFactory; the cache then stores typed runtime.Objects
// (e.g. *corev1.Pod) which are 2–4× smaller than the equivalent
// *unstructured.Unstructured representation. Otherwise it falls back to the
// dynamic informer, exactly as before.
func newInformerForIndex(dynamicClient dynamic.Interface, typedClient kubernetes.Interface, index FactoryIndex) (cache.SharedIndexInformer, bool) {
	resyncPeriod := randomizedResyncPeriod()

	tweakListOptions := func(options *metav1.ListOptions) {
		if index.FieldSelector != "" {
			options.FieldSelector = index.FieldSelector
		}
		if index.LabelSelector != "" {
			options.LabelSelector = index.LabelSelector
		}
	}

	if IsTypedInformerEnabled() && typedClient != nil {
		if _, ok := typedSupportedGVRs[index.GVR]; ok {
			typedFactory := informers.NewSharedInformerFactoryWithOptions(
				typedClient,
				resyncPeriod,
				informers.WithNamespace(index.Namespace),
				informers.WithTweakListOptions(tweakListOptions),
			)
			gi, err := typedFactory.ForResource(index.GVR)
			if err == nil {
				return gi.Informer(), true
			}
			log.Debug("typed informer not available for GVR, falling back to dynamic",
				slog.String(pkg.LogKeyGVR, index.GVR.String()),
				log.Err(err))
		}
	}

	dynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient, resyncPeriod, index.Namespace, tweakListOptions)
	return dynamicFactory.ForResource(index.GVR).Informer(), false
}

func (c *FactoryStore) get(ctx context.Context, dynamicClient dynamic.Interface, typedClient kubernetes.Interface, index FactoryIndex) *Factory {
	if f, ok := c.data[index]; ok {
		log.Debug("Factory store: the factory with index found",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		return f
	}

	informer, typed := newInformerForIndex(dynamicClient, typedClient, index)
	c.add(ctx, index, informer, typed)
	return c.data[index]
}

// Start is invoked by every resourceInformer that wants to receive events for
// the given index. It lazily materialises the underlying SharedIndexInformer
// the first time it sees an index, registers handler + transform + watch
// error handler, and reuses the same informer for every subsequent caller.
func (c *FactoryStore) Start(ctx context.Context, informerId string, dynamicClient dynamic.Interface, typedClient kubernetes.Interface, index FactoryIndex, handler cache.ResourceEventHandler, errorHandler *WatchErrorHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	factory := c.get(ctx, dynamicClient, typedClient, index)

	informer := factory.informer
	// Strip ManagedFields and last-applied-configuration before objects enter
	// the SharedIndexInformer cache. The transform is dispatched based on
	// whether the cache stores typed or unstructured payloads. Must be called
	// before Run(); subsequent invocations on the same informer are a harmless
	// no-op (returns "informer has already started" which we ignore).
	_ = informer.SetTransform(makeTransformFunc(factory.typed))
	// Add error handler, ignore "already started" error.
	_ = informer.SetWatchErrorHandler(errorHandler.handler)

	registration, err := informer.AddEventHandler(handler)
	if err != nil {
		log.Warn("Factory store: couldn't add event handler to the factory's informer",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()),
			log.Err(err))
	}

	factory.handlerRegistrations[informerId] = registration

	log.Debug("Factory store: increased usage counter of the factory",
		slog.Int(pkg.LogKeyValue, len(factory.handlerRegistrations)),
		slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

	// Ensure informer.Run is started once and tracked
	if factory.done == nil {
		factory.done = make(chan struct{})

		go func() {
			informer.Run(factory.ctx.Done())

			close(factory.done)

			log.Debug("Factory store: informer goroutine exited",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		}()
	}

	if !informer.HasSynced() {
		if err := wait.PollUntilContextCancel(ctx, DefaultSyncTime, true, func(_ context.Context) (bool, error) {
			return informer.HasSynced(), nil
		}); err != nil {
			return err
		}
	}

	log.Debug("Factory store: started informer",
		slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

	return nil
}

func (c *FactoryStore) Stop(informerId string, index FactoryIndex) {
	c.mu.Lock()
	f, ok := c.data[index]
	if !ok {
		// already deleted
		c.mu.Unlock()
		return
	}

	if handlerRegistration, found := f.handlerRegistrations[informerId]; found {
		err := f.informer.RemoveEventHandler(handlerRegistration)
		if err != nil {
			log.Warn("Factory store: couldn't remove event handler from the factory's informer",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()),
				log.Err(err))
		}

		delete(f.handlerRegistrations, informerId)

		log.Debug("Factory store: decreased usage counter of the factory",
			slog.Int(pkg.LogKeyValue, len(f.handlerRegistrations)),
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

		if len(f.handlerRegistrations) == 0 {
			log.Debug("Factory store: last handler removed, canceling shared informer",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

			done := f.done

			f.cancel()
			c.mu.Unlock()
			if done != nil {
				<-done
			}

			c.mu.Lock()
			delete(c.data, index)

			log.Debug("Factory store: deleted factory",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

			if ch, ok := c.stoppedCh[index]; ok {
				close(ch)
				delete(c.stoppedCh, index)
			}
		}
	}

	c.mu.Unlock()
}

// GetByKey returns the object cached at key inside the SharedIndexInformer
// associated with index. The key uses the standard client-go format
// "namespace/name" or "name" for cluster-scoped resources (see
// cache.MetaNamespaceKeyFunc).
//
// When the cache stores a typed runtime.Object (the typed-informer code path
// is enabled and the GVR is registered), the typed value is converted back to
// *unstructured.Unstructured here so callers can keep treating snapshot
// payloads uniformly.
//
// resourceInformer.cachedObjects stores only the slim filter result + checksum
// per object; the live *unstructured.Unstructured is resolved on demand via
// this method, which lets us avoid a second cache of object pointers.
func (c *FactoryStore) GetByKey(index FactoryIndex, key string) (*unstructured.Unstructured, bool) {
	c.mu.Lock()
	f, ok := c.data[index]
	c.mu.Unlock()
	if !ok || f == nil {
		return nil, false
	}
	item, exists, err := f.informer.GetStore().GetByKey(key)
	if err != nil || !exists || item == nil {
		return nil, false
	}
	obj := toUnstructured(item)
	if obj == nil {
		return nil, false
	}
	return obj, true
}

// WaitStopped blocks until the factory for index has been fully shut down (its
// informer goroutine has exited) or until FactoryShutdownTimeout elapses. If
// there is no factory for the index the function returns immediately.
func (c *FactoryStore) WaitStopped(index FactoryIndex) {
	c.mu.Lock()

	if _, ok := c.data[index]; !ok {
		c.mu.Unlock()
		return
	}

	ch, ok := c.stoppedCh[index]
	if !ok {
		// Register a channel so that Stop can signal us when the factory is gone.
		ch = make(chan struct{})
		c.stoppedCh[index] = ch
	}

	c.mu.Unlock()

	for {
		select {
		case <-ch:
			return
		case <-time.After(FactoryShutdownTimeout):
			log.Warn("timeout waiting for factory to stop",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		}
	}
}
