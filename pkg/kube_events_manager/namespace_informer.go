package kubeeventsmanager

// Namespace manager monitor namespaces for onKubernetesEvent config.

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	klient "github.com/flant/kube-client/client"
)

type namespaceInformer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool

	KubeClient     *klient.Client
	Monitor        *MonitorConfig
	SharedInformer cache.SharedInformer

	ExistedObjects map[string]bool

	addFn func(string)
	delFn func(string)
}

func NewNamespaceInformer(ctx context.Context, client *klient.Client, monitor *MonitorConfig) *namespaceInformer {
	cctx, cancel := context.WithCancel(ctx)

	informer := &namespaceInformer{
		ctx:            cctx,
		cancel:         cancel,
		KubeClient:     client,
		Monitor:        monitor,
		ExistedObjects: make(map[string]bool),
	}
	return informer
}

func (ni *namespaceInformer) withContext(ctx context.Context) {
	ni.ctx, ni.cancel = context.WithCancel(ctx)
}

func (ni *namespaceInformer) createSharedInformer(addFn func(string), delFn func(string)) error {
	// define resyncPeriod for informer
	resyncPeriod := randomizedResyncPeriod()

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	formatSelector, err := FormatLabelSelector(ni.Monitor.NamespaceSelector.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %s", ni.Monitor.NamespaceSelector.LabelSelector.String(), err)
	}
	tweakListOptions := func(options *metav1.ListOptions) {
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	ni.SharedInformer = corev1.NewFilteredNamespaceInformer(ni.KubeClient, resyncPeriod, indexers, tweakListOptions)
	ni.addFn = addFn
	ni.delFn = delFn
	ni.SharedInformer.AddEventHandler(ni) // SharedNamespaceInformerEventHandler(ni, addFn, delFn))

	listOptions := metav1.ListOptions{}
	tweakListOptions(&listOptions)
	existedObjects, err := ni.KubeClient.CoreV1().Namespaces().List(context.TODO(), listOptions)
	if err != nil {
		log.Error("Failed list existing namespaces", log.Err(err))
		return err
	}

	for _, ns := range existedObjects.Items {
		ni.ExistedObjects[ns.Name] = true
	}

	return nil
}

func (ni *namespaceInformer) getExistedObjects() map[string]bool {
	return ni.ExistedObjects
}

func (ni *namespaceInformer) OnAdd(obj interface{}, _ bool) {
	if ni.stopped {
		return
	}
	nsObj := obj.(*v1.Namespace)
	log.Debug("NamespaceInformer: Added ns", slog.String("name", nsObj.Name))
	if ni.addFn != nil {
		ni.addFn(nsObj.Name)
	}
}

func (ni *namespaceInformer) OnUpdate(_ interface{}, _ interface{}) {
	// Modified event for namespace is ignored
}

func (ni *namespaceInformer) OnDelete(obj interface{}) {
	if ni.stopped {
		return
	}
	if staleObj, stale := obj.(cache.DeletedFinalStateUnknown); stale {
		obj = staleObj.Obj
	}
	nsObj := obj.(*v1.Namespace)
	log.Debug("NamespaceInformer: Deleted ns", slog.String("name", nsObj.Name))
	if ni.delFn != nil {
		ni.delFn(nsObj.Name)
	}
}

func (ni *namespaceInformer) start() {
	log.Debug("Run namespace informer", slog.String("name", ni.Monitor.Metadata.DebugName))
	if ni.SharedInformer == nil {
		log.Error("Possible BUG!!! Start called before createSharedInformer, ShredInformer is nil",
			slog.String("debugName", ni.Monitor.Metadata.DebugName))
		return
	}
	cctx, cancel := context.WithCancel(ni.ctx)
	go func() {
		<-ni.ctx.Done()
		ni.stopped = true
		cancel()
	}()

	go ni.SharedInformer.Run(cctx.Done())

	if err := wait.PollUntilContextCancel(cctx, DefaultSyncTime, true, func(_ context.Context) (bool, error) {
		return ni.SharedInformer.HasSynced(), nil
	}); err != nil {
		ni.Monitor.Logger.Error("Cache is not synced for informer",
			slog.String("debugName", ni.Monitor.Metadata.DebugName))
	}

	log.Debug("Informer is ready", slog.String("debugName", ni.Monitor.Metadata.DebugName))
}

func (ni *namespaceInformer) pauseHandleEvents() {
	ni.stopped = true
}
