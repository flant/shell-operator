package kube_events_manager

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

var DefaultFactoryCache *FactoryCache

func init() {
	DefaultFactoryCache = NewFactoryCache()
}

type FactoryIndex struct {
	GVR           schema.GroupVersionResource
	Namespace     string
	FieldSelector string
	LabelSelector string
}

type FactoryCache struct {
	mu   sync.RWMutex
	data map[FactoryIndex]dynamicinformer.DynamicSharedInformerFactory
}

func NewFactoryCache() *FactoryCache {
	return &FactoryCache{
		data: make(map[FactoryIndex]dynamicinformer.DynamicSharedInformerFactory),
	}
}

func (c *FactoryCache) Get(index FactoryIndex) dynamicinformer.DynamicSharedInformerFactory {
	c.mu.RLock()
	defer c.mu.RUnlock()

	f, _ := c.data[index]
	return f
}

func (c *FactoryCache) Add(index FactoryIndex, f dynamicinformer.DynamicSharedInformerFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[index] = f
}
