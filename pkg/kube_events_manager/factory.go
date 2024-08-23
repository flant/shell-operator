package kube_events_manager

import (
	"context"
	"sync"
	"time"

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
	MonitorId     string
}

type Factory struct {
	shared dynamicinformer.DynamicSharedInformerFactory
	score  uint64
	ctx    context.Context
	cancel context.CancelFunc
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

func (c *FactoryStore) add(ctx context.Context, index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
	cctx, cancel := context.WithCancel(ctx)
	c.data[index] = Factory{
		shared: f,
		score:  uint64(1),
		ctx:    cctx,
		cancel: cancel,
	}
}

func (c *FactoryStore) get(ctx context.Context, client dynamic.Interface, index FactoryIndex) Factory {
	f, ok := c.data[index]
	if ok {
		f.score++
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

func (c *FactoryStore) Start(ctx context.Context, client dynamic.Interface, index FactoryIndex, handler cache.ResourceEventHandler, errorHandler *WatchErrorHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	factory := c.get(ctx, client, index)

	informer := factory.shared.ForResource(index.GVR).Informer()
	// Add error handler, ignore "already started" error.
	_ = informer.SetWatchErrorHandler(errorHandler.handler)
	// TODO(nabokihms): think about what will happen if we stop and then start the monitor
	informer.AddEventHandler(handler)

	if !informer.HasSynced() {
		go informer.Run(factory.ctx.Done())

		if err := wait.PollUntilContextCancel(ctx, DefaultSyncTime, true, func(_ context.Context) (bool, error) {
			return informer.HasSynced(), nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *FactoryStore) Stop(index FactoryIndex) {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, ok := c.data[index]
	if !ok {
		// already deleted
		return
	}

	f.score--
	if f.score == 0 {
		f.cancel()
	}

	delete(c.data, index)
}
