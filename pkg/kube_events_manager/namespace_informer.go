package kube_events_manager

// Namespace manager monitor namespaces for onKubernetesEvent config.

import (
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type NamespaceInformer interface {
	CreateSharedInformer(addFn func(string), delFn func(string)) error
	GetExistedObjects() map[string]bool
	Run(stopCh <-chan struct{})
	Stop()
}

type namespaceInformer struct {
	Monitor        *MonitorConfig
	SharedInformer cache.SharedInformer

	ExistedObjects map[string]bool
}

// namespaceInformer implements NamespaceInformer interface
var _ NamespaceInformer = &namespaceInformer{}

var NewNamespaceInformer = func(monitor *MonitorConfig) NamespaceInformer {
	informer := &namespaceInformer{
		Monitor:        monitor,
		ExistedObjects: make(map[string]bool, 0),
	}
	return informer
}

func (ni *namespaceInformer) CreateSharedInformer(addFn func(string), delFn func(string)) error {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

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

	ni.SharedInformer = corev1.NewFilteredNamespaceInformer(kube.Kubernetes, resyncPeriod, indexers, tweakListOptions)
	ni.SharedInformer.AddEventHandler(SharedNamespaceInformerEventHandler(ni, addFn, delFn))

	listOptions := metav1.ListOptions{}
	tweakListOptions(&listOptions)
	existedObjects, err := kube.Kubernetes.CoreV1().Namespaces().List(listOptions)

	if err != nil {
		log.Errorf("list existing namespaces: %v", err)
		return err
	}

	for _, ns := range existedObjects.Items {
		ni.ExistedObjects[ns.Name] = true
	}

	return nil
}

func (ni *namespaceInformer) GetExistedObjects() map[string]bool {
	return ni.ExistedObjects
}

var SharedNamespaceInformerEventHandler = func(informer *namespaceInformer, addFn func(string), delFn func(string)) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nsObj := obj.(*v1.Namespace)
			log.Debugf("NamespaceInformer: Added ns/%s", nsObj.Name)
			addFn(nsObj.Name)
		},
		DeleteFunc: func(obj interface{}) {
			nsObj := obj.(*v1.Namespace)
			log.Debugf("NamespaceInformer: Deleted ns/%s", nsObj.Name)
			delFn(nsObj.Name)
		},
	}
}

func (ni *namespaceInformer) Run(stopCh <-chan struct{}) {
	log.Debugf("%s: Run namespace informer", ni.Monitor.Metadata.DebugName)
	if ni.SharedInformer != nil {
		go ni.SharedInformer.Run(stopCh)
	}
}

func (ni *namespaceInformer) Stop() {
	return
}
