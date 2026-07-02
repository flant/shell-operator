package kubeeventsmanager

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
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

type Factory struct {
	shared               dynamicinformer.DynamicSharedInformerFactory
	handlerRegistrations map[string]cache.ResourceEventHandlerRegistration
	ctx                  context.Context
	cancel               context.CancelFunc
	// done is closed when the underlying informer.Run returns
	done chan struct{}
}

type FactoryStore struct {
	mu   sync.Mutex
	data map[FactoryIndex]*Factory
	// baseCtx is the lifetime anchor for every shared informer created by the
	// store. Shared informers are long-lived, store-owned resources, so their
	// contexts must descend from baseCtx (tied to the events-manager), NOT from
	// the transient context of whichever consumer happened to register first.
	// Otherwise cancelling one consumer's context would tear down the shared
	// informer for every other consumer still using it.
	baseCtx   context.Context
	stoppedCh map[FactoryIndex]chan struct{}
}

func NewFactoryStore(ctx context.Context) *FactoryStore {
	fs := &FactoryStore{
		data:      make(map[FactoryIndex]*Factory),
		baseCtx:   ctx,
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

func (c *FactoryStore) add(index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
	// Derive from the store's baseCtx, not from the caller's context: the
	// shared informer's lifetime is owned by the store and must only end when
	// the last handler is removed (see Stop) or the manager shuts down.
	ctx, cancel := context.WithCancel(c.baseCtx)
	c.data[index] = &Factory{
		shared:               f,
		handlerRegistrations: make(map[string]cache.ResourceEventHandlerRegistration),
		ctx:                  ctx,
		cancel:               cancel,
		done:                 nil,
	}

	log.Debug("Factory store: added a new factory for index",
		slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
}

// isDead reports whether the factory's shared informer goroutine has already
// exited (its Run returned and done was closed). A dead factory must never be
// reused: attaching a handler to it silently succeeds while the informer never
// lists/watches again, freezing the snapshot until the process restarts.
func (f *Factory) isDead() bool {
	if f.done == nil {
		return false
	}
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

func (c *FactoryStore) get(client dynamic.Interface, index FactoryIndex) *Factory {
	f, ok := c.data[index]
	if ok && !f.isDead() {
		log.Debug("Factory store: the factory with index found",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		return f
	}
	if ok {
		// The cached factory's informer has terminated (e.g. torn down in the
		// window of a concurrent Stop). Discard the corpse and rebuild instead
		// of handing back a dead informer.
		log.Warn("Factory store: cached factory is dead, recreating",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		f.cancel()
		delete(c.data, index)
	}

	// define resyncPeriod for informer
	resyncPeriod := randomizedResyncPeriod()

	tweakListOptions := func(options *metav1.ListOptions) {
		if index.FieldSelector != "" {
			options.FieldSelector = index.FieldSelector
		}
		if index.LabelSelector != "" {
			options.LabelSelector = index.LabelSelector
		}
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		client, resyncPeriod, index.Namespace, tweakListOptions)
	factory.ForResource(index.GVR)

	c.add(index, factory)

	return c.data[index]
}

func (c *FactoryStore) Start(ctx context.Context, informerId string, client dynamic.Interface, index FactoryIndex, handler cache.ResourceEventHandler, errorHandler *WatchErrorHandler) error {
	c.mu.Lock()

	factory := c.get(client, index)

	informer := factory.shared.ForResource(index.GVR).Informer()
	// Register the watch error handler. This returns an error when the shared
	// informer is already running (a previous consumer registered its handler);
	// that is expected on a reused factory, so log rather than silently drop it.
	if err := informer.SetWatchErrorHandler(errorHandler.handler); err != nil {
		log.Debug("Factory store: couldn't set watch error handler, informer likely already started",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()),
			log.Err(err))
	}

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

	// Release the store lock before waiting for the initial sync. Holding c.mu
	// across a blocking cache sync would serialize Start/Stop of every other
	// informer behind a single slow initial LIST/discovery (throttled API,
	// heavy CRD). The informer handle is safe to poll without the lock.
	c.mu.Unlock()

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
		err := f.shared.ForResource(index.GVR).Informer().RemoveEventHandler(handlerRegistration)
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
			// Only remove the factory if it is still the same instance we just
			// cancelled. While the lock was released (waiting on done), a
			// concurrent Start could have observed this dead factory, rebuilt
			// it, and installed a fresh, live one under the same index — that
			// one must not be deleted, or its informer goroutine would be
			// orphaned and untracked.
			if cur, ok := c.data[index]; ok && cur == f {
				delete(c.data, index)

				log.Debug("Factory store: deleted factory",
					slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
			}

			if ch, ok := c.stoppedCh[index]; ok {
				close(ch)
				delete(c.stoppedCh, index)
			}
		}
	}

	c.mu.Unlock()
}

// WaitStopped blocks until there is no factory for the index or timeout
func (c *FactoryStore) WaitStopped(index FactoryIndex) {
	c.mu.Lock()

	if _, ok := c.data[index]; !ok {
		c.mu.Unlock()
		return
	}

	ch, ok := c.stoppedCh[index]
	if !ok {
		ch = make(chan struct{})
		close(ch)
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
