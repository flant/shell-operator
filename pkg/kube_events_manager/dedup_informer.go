package kubeeventsmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/ldmonster/kubeclient/store"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	clientgocache "k8s.io/client-go/tools/cache"
)

type dedupResourceInformer struct {
	index  FactoryIndex
	client dynamic.Interface
	store  store.Store

	mu        sync.RWMutex
	handlers  map[string]clientgocache.ResourceEventHandler
	hasSynced bool
}

func newDedupResourceInformer(client dynamic.Interface, index FactoryIndex, dedupStore store.Store) *dedupResourceInformer {
	if dedupStore == nil {
		dedupStore = store.NewDedupStore()
	}
	return &dedupResourceInformer{
		index:    index,
		client:   client,
		store:    dedupStore,
		handlers: make(map[string]clientgocache.ResourceEventHandler),
	}
}

func (inf *dedupResourceInformer) addEventHandler(id string, handler clientgocache.ResourceEventHandler) {
	inf.mu.Lock()
	defer inf.mu.Unlock()
	inf.handlers[id] = handler
}

func (inf *dedupResourceInformer) removeEventHandler(id string) {
	inf.mu.Lock()
	defer inf.mu.Unlock()
	delete(inf.handlers, id)
}

func (inf *dedupResourceInformer) handlerCount() int {
	inf.mu.RLock()
	defer inf.mu.RUnlock()
	return len(inf.handlers)
}

func (inf *dedupResourceInformer) synced() bool {
	inf.mu.RLock()
	defer inf.mu.RUnlock()
	return inf.hasSynced
}

func (inf *dedupResourceInformer) run(ctx context.Context, errorHandler *WatchErrorHandler) error {
	resourceVersion, err := inf.list(ctx, true)
	if err != nil {
		return fmt.Errorf("dedup informer: initial list failed for %s: %w", inf.index.GVR.String(), err)
	}

	inf.mu.Lock()
	inf.hasSynced = true
	inf.mu.Unlock()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		nextResourceVersion, err := inf.watchLoop(ctx, resourceVersion)
		if err == nil {
			return ctx.Err()
		}
		inf.reportWatchError(errorHandler, err)
		if nextResourceVersion != "" {
			resourceVersion = nextResourceVersion
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		resourceVersion, err = inf.list(ctx, true)
		if err != nil {
			inf.reportWatchError(errorHandler, err)
			continue
		}
		backoff = time.Second
	}
}

func (inf *dedupResourceInformer) list(ctx context.Context, notify bool) (string, error) {
	listResult, err := inf.resource().List(ctx, inf.listOptions(""))
	if err != nil {
		return "", fmt.Errorf("list failed: %w", err)
	}

	seen := make(map[store.ObjectKey]struct{}, len(listResult.Items))
	for i := range listResult.Items {
		obj := listResult.Items[i].DeepCopy()
		inf.prepareObject(obj)
		key := inf.objectKey(obj)
		seen[key] = struct{}{}

		oldObj, hadOld := inf.store.Get(key)
		if err := inf.store.Upsert(key, obj); err != nil {
			return "", fmt.Errorf("store upsert failed: %w", err)
		}

		if notify {
			if hadOld {
				inf.notifyUpdate(oldObj, obj)
			} else {
				inf.notifyAdd(obj)
			}
		}
	}

	for _, oldObj := range inf.store.List(inf.index.GVK) {
		inf.prepareObject(oldObj)
		key := inf.objectKey(oldObj)
		if _, ok := seen[key]; ok {
			continue
		}
		if err := inf.store.Delete(key); err != nil {
			return "", fmt.Errorf("store delete failed: %w", err)
		}
		if notify {
			inf.notifyDelete(oldObj)
		}
	}

	return listResult.GetResourceVersion(), nil
}

func (inf *dedupResourceInformer) watchLoop(ctx context.Context, resourceVersion string) (string, error) {
	watcher, err := inf.resource().Watch(ctx, inf.listOptions(resourceVersion))
	if err != nil {
		return resourceVersion, fmt.Errorf("watch failed: %w", err)
	}
	defer watcher.Stop()

	currentResourceVersion := resourceVersion
	for {
		select {
		case <-ctx.Done():
			return currentResourceVersion, nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return currentResourceVersion, io.EOF
			}
			nextResourceVersion, err := inf.handleWatchEvent(event)
			if nextResourceVersion != "" {
				currentResourceVersion = nextResourceVersion
			}
			if err != nil {
				return currentResourceVersion, err
			}
		}
	}
}

func (inf *dedupResourceInformer) handleWatchEvent(event watch.Event) (string, error) {
	resourceVersion := resourceVersionOf(event.Object)

	switch event.Type {
	case watch.Bookmark:
		return resourceVersion, nil
	case watch.Error:
		return resourceVersion, errorFromWatchObject(event.Object)
	}

	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return resourceVersion, fmt.Errorf("unexpected watch object type %T", event.Object)
	}
	obj = obj.DeepCopy()
	inf.prepareObject(obj)
	key := inf.objectKey(obj)

	switch event.Type {
	case watch.Added:
		if err := inf.store.Upsert(key, obj); err != nil {
			return resourceVersion, fmt.Errorf("store upsert failed: %w", err)
		}
		inf.notifyAdd(obj)
	case watch.Modified:
		oldObj, hadOld := inf.store.Get(key)
		if err := inf.store.Upsert(key, obj); err != nil {
			return resourceVersion, fmt.Errorf("store upsert failed: %w", err)
		}
		if hadOld {
			inf.notifyUpdate(oldObj, obj)
		} else {
			inf.notifyAdd(obj)
		}
	case watch.Deleted:
		if err := inf.store.Delete(key); err != nil {
			return resourceVersion, fmt.Errorf("store delete failed: %w", err)
		}
		inf.notifyDelete(obj)
	default:
		return resourceVersion, fmt.Errorf("unexpected watch event type %q", event.Type)
	}

	return resourceVersion, nil
}

func (inf *dedupResourceInformer) resource() dynamic.ResourceInterface {
	resource := inf.client.Resource(inf.index.GVR)
	if inf.index.Namespace == "" {
		return resource
	}
	return resource.Namespace(inf.index.Namespace)
}

func (inf *dedupResourceInformer) listOptions(resourceVersion string) metav1.ListOptions {
	options := metav1.ListOptions{
		FieldSelector: inf.index.FieldSelector,
		LabelSelector: inf.index.LabelSelector,
	}
	if resourceVersion != "" {
		options.ResourceVersion = resourceVersion
	}
	return options
}

func (inf *dedupResourceInformer) objectKey(obj *unstructured.Unstructured) store.ObjectKey {
	return store.ObjectKey{
		GVK:       inf.index.GVK,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func (inf *dedupResourceInformer) prepareObject(obj *unstructured.Unstructured) {
	obj.SetGroupVersionKind(inf.index.GVK)
}

func (inf *dedupResourceInformer) handlersSnapshot() []clientgocache.ResourceEventHandler {
	inf.mu.RLock()
	defer inf.mu.RUnlock()

	handlers := make([]clientgocache.ResourceEventHandler, 0, len(inf.handlers))
	for _, handler := range inf.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

func (inf *dedupResourceInformer) notifyAdd(obj *unstructured.Unstructured) {
	for _, handler := range inf.handlersSnapshot() {
		handler.OnAdd(obj.DeepCopy(), false)
	}
}

func (inf *dedupResourceInformer) notifyUpdate(oldObj, newObj *unstructured.Unstructured) {
	for _, handler := range inf.handlersSnapshot() {
		handler.OnUpdate(oldObj.DeepCopy(), newObj.DeepCopy())
	}
}

func (inf *dedupResourceInformer) notifyDelete(obj *unstructured.Unstructured) {
	for _, handler := range inf.handlersSnapshot() {
		handler.OnDelete(obj.DeepCopy())
	}
}

func (inf *dedupResourceInformer) reportWatchError(errorHandler *WatchErrorHandler, err error) {
	if errorHandler != nil && err != nil && !errors.Is(err, context.Canceled) {
		errorHandler.handler(nil, err)
		return
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Debug("dedup informer watch error", slog.String("error", err.Error()))
	}
}

func resourceVersionOf(obj runtime.Object) string {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	return accessor.GetResourceVersion()
}

func errorFromWatchObject(obj runtime.Object) error {
	status, ok := obj.(*metav1.Status)
	if !ok {
		return fmt.Errorf("watch returned error object %T", obj)
	}
	return apierrors.FromObject(status)
}
