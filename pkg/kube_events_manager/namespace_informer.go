package kube_events_manager

// Namespace manager monitor namespaces for onKubernetesEvent config.

import (
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type NamespaceInformer interface {
	CreateSharedInformer(addFn func(string), delFn func(string)) error
	Run(stopCh <-chan struct{})
	Stop()
}

type namespaceInformer struct {
	Monitor        *MonitorConfig
	SharedInformer cache.SharedInformer
}

// namespaceInformer implements NamespaceInformer interface
var _ NamespaceInformer = &namespaceInformer{}

var NewNamespaceInformer = func(monitor *MonitorConfig) NamespaceInformer {
	informer := &namespaceInformer{
		Monitor: monitor,
	}
	return informer
}

func (m *namespaceInformer) CreateSharedInformer(addFn func(string), delFn func(string)) error {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	formatSelector, err := FormatLabelSelector(m.Monitor.NamespaceSelector.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %s", m.Monitor.NamespaceSelector.LabelSelector.String(), err)
	}
	tweakListOptions := func(options *metav1.ListOptions) {
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	m.SharedInformer = corev1.NewFilteredNamespaceInformer(kube.Kubernetes, resyncPeriod, indexers, tweakListOptions)
	m.SharedInformer.AddEventHandler(SharedNamespaceInformerEventHandler(m, addFn, delFn))
	//resourceList, listErr = kube.Kubernetes.CoreV1().Namespaces().List(listOptions)

	return nil
}

var SharedNamespaceInformerEventHandler = func(informer *namespaceInformer, addFn func(string), delFn func(string)) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			_, nsName, err := metaFromEventObject(obj)
			if err != nil {
				log.Errorf("%s: add: get ns name: %s", informer.Monitor.Metadata.DebugName, err)
				return
			}

			addFn(nsName)
		},
		DeleteFunc: func(obj interface{}) {
			_, nsName, err := metaFromEventObject(obj)
			if err != nil {
				log.Errorf("%s: delete: get ns name: %s", informer.Monitor.Metadata.DebugName, err)
				return
			}

			delFn(nsName)
		},
	}
}

func (m *namespaceInformer) Run(stopCh <-chan struct{}) {
	log.Debugf("%s: Run namespace informer", m.Monitor.Metadata.DebugName)
	if m.SharedInformer != nil {
		go m.SharedInformer.Run(stopCh)
	}
}

func (m *namespaceInformer) Stop() {
	return
}
