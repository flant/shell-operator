package kube_events_manager

// Namespace manager monitor namespaces for onKubernetesEvent config.

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
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

	KubeClient     klient.Client
	Monitor        *MonitorConfig
	SharedInformer cache.SharedInformer

	ExistedObjects map[string]bool

	addFn func(string)
	delFn func(string)
}

func NewNamespaceInformer(ctx context.Context, client klient.Client, monitor *MonitorConfig) *namespaceInformer {
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
		log.Errorf("list existing namespaces: %v", err)
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

func (ni *namespaceInformer) OnAdd(obj interface{}) {
	if ni.stopped {
		return
	}
	nsObj := obj.(*v1.Namespace)
	log.Debugf("NamespaceInformer: Added ns/%s", nsObj.Name)
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
	log.Debugf("NamespaceInformer: Deleted ns/%s", nsObj.Name)
	if ni.delFn != nil {
		ni.delFn(nsObj.Name)
	}
}

func (ni *namespaceInformer) start() {
	log.Debugf("%s: Run namespace informer", ni.Monitor.Metadata.DebugName)
	if ni.SharedInformer == nil {
		log.Errorf("%s: Possible BUG!!! Start called before createSharedInformer, ShredInformer is nil", ni.Monitor.Metadata.DebugName)
		return
	}
	stopCh := make(chan struct{}, 1)
	go func() {
		<-ni.ctx.Done()
		ni.stopped = true
		close(stopCh)
	}()

	go ni.SharedInformer.Run(stopCh)
	if err := wait.PollImmediateUntil(DefaultSyncTime, func() (bool, error) {
		return ni.SharedInformer.HasSynced(), nil
	}, stopCh); err != nil {
		ni.Monitor.LogEntry.Errorf("%s: cache is not synced for informer", ni.Monitor.Metadata.DebugName)
	}

	log.Debugf("%s: informer is ready", ni.Monitor.Metadata.DebugName)
}

func (ni *namespaceInformer) pauseHandleEvents() {
	ni.stopped = true
}
