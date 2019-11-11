package kube_events_manager

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

type Monitor interface {
	WithName(string)
	WithConfig(config *MonitorConfig)
	CreateInformers(logEntry *log.Entry) error
	GetExistedObjects() []ObjectAndFilterResult
	Start(context.Context)
	Stop()
}

// Monitor holds informers for resources and a namespace informer
type monitor struct {
	Name   string
	Config *MonitorConfig
	// Static list of informers
	ResourceInformers []ResourceInformer
	// Namespace informer to get new namespaces
	NamespaceInformer NamespaceInformer
	// map of dynamically starting informers
	VaryingInformers map[string][]ResourceInformer

	// Index of namespaces statically defined in monitor configuration
	staticNamespaces map[string]bool

	cancelForNs map[string]context.CancelFunc

	ctx    context.Context
	cancel context.CancelFunc

	l sync.Mutex
}

var NewMonitor = func() Monitor {
	return &monitor{
		ResourceInformers: make([]ResourceInformer, 0),
		VaryingInformers:  make(map[string][]ResourceInformer, 0),
		cancelForNs:       make(map[string]context.CancelFunc, 0),
		staticNamespaces:  make(map[string]bool, 0),
	}
}

func (m *monitor) WithName(name string) {
	m.Name = name
}

func (m *monitor) WithConfig(config *MonitorConfig) {
	m.Config = config
}

// CreateInformers creates all informers and
// a namespace informer if namespace.labelSelector is defined.
// If MonitorConfig.NamespaceSelector.MatchNames is defined, then
// multiple informers are created for each namespace.
// If no NamespaceSelector defined, then one informer is created.
func (m *monitor) CreateInformers(logEntry *log.Entry) error {
	logEntry = logEntry.WithField("binding.name", m.Config.Metadata.DebugName)
	logEntry.Debugf("Create Informers Config: %+v", m.Config)
	nsNames := m.Config.Namespaces()
	if len(nsNames) > 0 {
		logEntry.Debugf("create static ResourceInformers")

		// create informers for each specified object name in each specified namespace
		// This list of informers is static.
		for _, nsName := range nsNames {
			if nsName != "" {
				m.staticNamespaces[nsName] = true
			}
			informers, err := m.CreateInformersForNamespace(nsName)
			if err != nil {
				return err
			}
			m.ResourceInformers = append(m.ResourceInformers, informers...)
		}
	}

	if m.Config.NamespaceSelector != nil && m.Config.NamespaceSelector.LabelSelector != nil {
		logEntry.Debugf("Create NamespaceInformer for namespace.labelSelector")
		m.NamespaceInformer = NewNamespaceInformer(m.Config)
		err := m.NamespaceInformer.CreateSharedInformer(
			func(nsName string) {
				// add function — check, create and run informers for Ns
				logEntry.Infof("got ns/%s, create dynamic ResourceInformers", nsName)

				// ignore event if namespace is already has static ResourceInformers
				if _, ok := m.staticNamespaces[nsName]; ok {
					return
				}
				// ignore already started informers
				_, ok := m.VaryingInformers[nsName]
				if ok {
					return
				}

				var err error
				m.VaryingInformers[nsName], err = m.CreateInformersForNamespace(nsName)
				if err != nil {
					logEntry.Errorf("create ResourceInformers for ns/%s: %v", nsName, err)
				}

				var ctx context.Context
				ctx, m.cancelForNs[nsName] = context.WithCancel(m.ctx)

				for _, informer := range m.VaryingInformers[nsName] {
					go informer.Run(ctx.Done())
				}
			},
			func(nsName string) {
				// delete function — check, stop and remove informers for Ns
				logEntry.Infof("deleted ns/%s, stop dynamic ResourceInformers", nsName)

				// ignore statically specified namespaces
				if _, ok := m.staticNamespaces[nsName]; ok {
					return
				}

				// ignore already stopped informers
				_, ok := m.cancelForNs[nsName]
				if !ok {
					return
				}

				m.cancelForNs[nsName]()

				// TODO wait

				delete(m.VaryingInformers, nsName)
				delete(m.cancelForNs, nsName)
			},
		)
		if err != nil {
			return fmt.Errorf("create namespace informer: %v", err)
		}
		for nsName := range m.NamespaceInformer.GetExistedObjects() {
			logEntry.Infof("got ns/%s, create dynamic ResourceInformers", nsName)

			// ignore event if namespace is already has static ResourceInformers
			if _, ok := m.staticNamespaces[nsName]; ok {
				continue
			}

			var err error
			m.VaryingInformers[nsName], err = m.CreateInformersForNamespace(nsName)
			if err != nil {
				logEntry.Errorf("create ResourceInformers for ns/%s: %v", nsName, err)
			}
		}
	}

	return nil
}

// GetExistedObjects returns all existed objects from all created informers
func (m *monitor) GetExistedObjects() []ObjectAndFilterResult {
	objects := make([]ObjectAndFilterResult, 0)

	for _, informer := range m.ResourceInformers {
		objects = append(objects, informer.GetExistedObjects()...)
	}

	for nsName := range m.VaryingInformers {
		for _, informer := range m.VaryingInformers[nsName] {
			objects = append(objects, informer.GetExistedObjects()...)
		}
	}

	return objects
}

// CreateInformersForNamespace creates informers bounded to the namespace. If no matchName is specified,
// it is only one informer. If matchName is specified, then multiple informers are created.
//
// If namespace is empty, then informer is bounded to all namespaces.
func (m *monitor) CreateInformersForNamespace(namespace string) (informers []ResourceInformer, err error) {
	informers = make([]ResourceInformer, 0)

	objNames := []string{""}

	if len(m.Config.Names()) > 0 {
		objNames = m.Config.Names()
	}

	for _, objName := range objNames {
		informer := NewResourceInformer(m.Config)
		informer.WithNamespace(namespace)
		informer.WithName(objName)

		err := informer.CreateSharedInformer()
		if err != nil {
			return nil, err
		}

		informers = append(informers, informer)
	}
	return informers, nil
}

// Start calls Run on all informers.
func (m *monitor) Start(parentCtx context.Context) {
	m.ctx, m.cancel = context.WithCancel(parentCtx)

	for _, informer := range m.ResourceInformers {
		go informer.Run(m.ctx.Done())
	}

	for nsName := range m.VaryingInformers {
		var ctx context.Context
		ctx, m.cancelForNs[nsName] = context.WithCancel(m.ctx)
		for _, informer := range m.VaryingInformers[nsName] {
			go informer.Run(ctx.Done())
		}
	}

	if m.NamespaceInformer != nil {
		go m.NamespaceInformer.Run(m.ctx.Done())
	}

	return
}

// Stop stops all informers
func (m *monitor) Stop() {
	m.cancel()
}
