package kube_events_manager

import (
	"context"
	"sync"
	"time"

	log "github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
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
}

type FactoryStore struct {
	mu   sync.Mutex
	data map[FactoryIndex]Factory
}

func NewFactoryStore() *FactoryStore {
	return &FactoryStore{
		data: make(map[FactoryIndex]Factory),
	}
}

func (c *FactoryStore) add(index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
	ctx, cancel := context.WithCancel(context.Background())
	c.data[index] = Factory{
		shared:               f,
		handlerRegistrations: make(map[string]cache.ResourceEventHandlerRegistration, 0),
		ctx:                  ctx,
		cancel:               cancel,
	}
	log.Debugf("Factory store: added a new factory for %v index", index)
}

func (c *FactoryStore) get(client dynamic.Interface, index FactoryIndex) Factory {
	f, ok := c.data[index]
	if ok {
		log.Debugf("Factory store: the factory with %v index found", index)
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
	defer c.mu.Unlock()

	factory := c.get(client, index)

	informer := factory.shared.ForResource(index.GVR).Informer()
	// Add error handler, ignore "already started" error.
	_ = informer.SetWatchErrorHandler(errorHandler.handler)
	registration, err := informer.AddEventHandler(handler)
	if err != nil {
		log.Warnf("Factory store: couldn't add event handler to the %v factory's informer: %v", index, err)
	}
	factory.handlerRegistrations[informerId] = registration
	log.Debugf("Factory store: increased usage counter to %d of the factory with %v index", len(factory.handlerRegistrations), index)

	if !informer.HasSynced() {
		go informer.Run(factory.ctx.Done())

		if err := wait.PollUntilContextCancel(ctx, DefaultSyncTime, true, func(_ context.Context) (bool, error) {
			return informer.HasSynced(), nil
		}); err != nil {
			return err
		}
	}
	log.Debugf("Factory store: started informer for %v index", index)
	return nil
}

func (c *FactoryStore) Stop(informerId string, index FactoryIndex) {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, ok := c.data[index]
	if !ok {
		// already deleted
		return
	}

	if handlerRegistration, found := f.handlerRegistrations[informerId]; found {
		err := f.shared.ForResource(index.GVR).Informer().RemoveEventHandler(handlerRegistration)
		if err != nil {
			log.Warnf("Factory store: couldn't remove event handler from the %v factory's informer: %v", index, err)
		}
		delete(f.handlerRegistrations, informerId)
		log.Debugf("Factory store: decreased usage counter to %d of the factory with %v index", len(f.handlerRegistrations), index)
		if len(f.handlerRegistrations) == 0 {
			f.cancel()
			delete(c.data, index)
			log.Debugf("Factory store: deleted factory for %v index", index)
		}
	}
}
