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
)

const (
	FactoryShutdownTimeout = 30 * time.Second
)

var (
	DefaultFactoryStore *FactoryStore
	DefaultSyncTime     = 100 * time.Millisecond
)

func init() {
	DefaultFactoryStore = NewFactoryStore()
}

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

func (c *FactoryStore) add(ctx context.Context, index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
	ctx, cancel := context.WithCancel(ctx)
	c.data[index] = &Factory{
		shared:               f,
		handlerRegistrations: make(map[string]cache.ResourceEventHandlerRegistration),
		ctx:                  ctx,
		cancel:               cancel,
		done:                 nil,
	}

	log.Debug("Factory store: added a new factory for index",
		slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))
}

func (c *FactoryStore) get(ctx context.Context, client dynamic.Interface, index FactoryIndex) *Factory {
	f, ok := c.data[index]
	if ok {
		log.Debug("Factory store: the factory with index found",
			slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))
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

	c.add(ctx, index, factory)

	return c.data[index]
}

func (c *FactoryStore) Start(ctx context.Context, informerId string, client dynamic.Interface, index FactoryIndex, handler cache.ResourceEventHandler, errorHandler *WatchErrorHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	factory := c.get(ctx, client, index)

	informer := factory.shared.ForResource(index.GVR).Informer()
	// Add error handler, ignore "already started" error.
	_ = informer.SetWatchErrorHandler(errorHandler.handler)

	registration, err := informer.AddEventHandler(handler)
	if err != nil {
		log.Warn("Factory store: couldn't add event handler to the factory's informer",
			slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()),
			log.Err(err))
	}

	factory.handlerRegistrations[informerId] = registration

	log.Debug("Factory store: increased usage counter of the factory",
		slog.Int("value", len(factory.handlerRegistrations)),
		slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))

	// Ensure informer.Run is started once and tracked
	if factory.done == nil {
		factory.done = make(chan struct{})

		go func() {
			informer.Run(factory.ctx.Done())

			close(factory.done)

			log.Debug("Factory store: informer goroutine exited",
				slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))
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
		slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))

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
				slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()),
				log.Err(err))
		}

		delete(f.handlerRegistrations, informerId)

		log.Debug("Factory store: decreased usage counter of the factory",
			slog.Int("value", len(f.handlerRegistrations)),
			slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))

		if len(f.handlerRegistrations) == 0 {
			log.Debug("Factory store: last handler removed, canceling shared informer",
				slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))

			done := f.done

			f.cancel()
			c.mu.Unlock()
			if done != nil {
				<-done
			}

			c.mu.Lock()
			delete(c.data, index)

			log.Debug("Factory store: deleted factory",
				slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))

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
		c.stoppedCh[index] = ch
	}

	c.mu.Unlock()

	for {
		select {
		case <-ch:
			return
		case <-time.After(FactoryShutdownTimeout):
			log.Warn("timeout waiting for factory to stop",
				slog.String("namespace", index.Namespace), slog.String("gvr", index.GVR.String()))
		}
	}
}
