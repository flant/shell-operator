package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/metrics"
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
	mu sync.Mutex
	// baseCtx bounds the lifetime of every shared informer in the store. The store
	// (via its owning kubeEventsManager) is the owner of the shared factories, so
	// their contexts must derive from the owner's context — never from the context
	// of whichever consumer happened to create a factory first, or stopping that
	// consumer would kill the shared informer for every other consumer.
	baseCtx   context.Context
	data      map[FactoryIndex]*Factory
	stoppedCh map[FactoryIndex]chan struct{}
	// optional, only for the factory_informer_dead_total counter
	metricStorage metricsstorage.Storage
}

func NewFactoryStore(ctx context.Context) *FactoryStore {
	if ctx == nil {
		ctx = context.Background()
	}

	fs := &FactoryStore{
		baseCtx:   ctx,
		data:      make(map[FactoryIndex]*Factory),
		stoppedCh: make(map[FactoryIndex]chan struct{}),
	}
	return fs
}

// WithMetricStorage sets an optional storage for the factory_informer_dead_total
// counter. Without it a dead factory is still logged and recreated, just not counted.
func (c *FactoryStore) WithMetricStorage(mstor metricsstorage.Storage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metricStorage = mstor
}

func (c *FactoryStore) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Stop informer goroutines before dropping the references, otherwise they
	// keep running detached forever.
	for _, f := range c.data {
		f.cancel()
	}
	c.data = make(map[FactoryIndex]*Factory)
	c.stoppedCh = make(map[FactoryIndex]chan struct{})
}

func (c *FactoryStore) add(index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
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

func (c *FactoryStore) get(client dynamic.Interface, index FactoryIndex) *Factory {
	f, ok := c.data[index]
	if ok {
		log.Debug("Factory store: the factory with index found",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))
		return f
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

	// A factory whose informer goroutine has already exited is unusable: event
	// handlers can no longer be added and no events will ever be delivered, while
	// HasSynced() on the stopped informer still returns true. Reusing it would
	// hand the consumer a snapshot that silently never updates. Drop the corpse
	// and build a fresh factory instead.
	if factory.done != nil {
		select {
		case <-factory.done:
			log.Warn("Factory store: informer for index is stopped, recreating factory",
				slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()))

			// A dead factory in the store means some consumer lived with a frozen
			// snapshot until this Start; make each detection countable.
			if c.metricStorage != nil {
				c.metricStorage.CounterAdd(metrics.FactoryInformerDeadTotal, 1.0, map[string]string{
					"gvr":       index.GVR.String(),
					"namespace": index.Namespace,
				})
			}

			factory.cancel()
			delete(c.data, index)
			factory = c.get(client, index)
		default:
		}
	}

	informer := factory.shared.ForResource(index.GVR).Informer()
	// SetWatchErrorHandler fails only on an already started informer — harmless
	// here, the handler set by the first consumer stays in effect.
	if err := informer.SetWatchErrorHandler(errorHandler.handler); err != nil {
		log.Debug("Factory store: watch error handler wasn't set",
			slog.String(pkg.LogKeyNamespace, index.Namespace), slog.String(pkg.LogKeyGVR, index.GVR.String()),
			log.Err(err))
	}

	registration, err := informer.AddEventHandler(handler)
	if err != nil {
		// A consumer without a registered handler would run with a snapshot that
		// never updates — fail the start instead of degrading silently.
		c.mu.Unlock()
		return fmt.Errorf("add event handler for %s: %w", index.GVR.String(), err)
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

	c.mu.Unlock()

	// Wait for the cache sync outside the store lock: a slow initial LIST of one
	// resource must not serialize Start/Stop of every other informer (same
	// unlock-before-wait pattern as in Stop).
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
