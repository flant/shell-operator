package kube_events_manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/romana/rlog"
)

type Monitor interface {
	WithName(string)
	WithConfig(config *MonitorConfig)
	CreateInformers() error
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

	cancelForNs map[string]context.CancelFunc

	ctx    context.Context
	cancel context.CancelFunc

	l sync.Mutex
}

var NewMonitor = func() Monitor {
	return &monitor{
		ResourceInformers: make([]ResourceInformer, 0),
		VaryingInformers:  make(map[string][]ResourceInformer),
		cancelForNs:       make(map[string]context.CancelFunc),
	}
}

func (m *monitor) WithName(name string) {
	m.Name = name
}

func (m *monitor) WithConfig(config *MonitorConfig) {
	rlog.Debugf("WithConfig: %+v", config)
	m.Config = config
	rlog.Debugf("WithConfig: %+v", m.Config)
}

// CreateInformers creates all informers and
// a namespace informer if namespace.labelSelector is defined.
// If MonitorConfig.NamespaceSelector.MatchNames is defined, then
// multiple informers are created for each namespace.
// If no NamespaceSelector defined, then one informer is created.
func (m *monitor) CreateInformers() error {
	rlog.Debugf("Create Informers Config: %+v", m.Config)
	nsNames := m.Config.Namespaces()
	if len(nsNames) > 0 {
		rlog.Debugf("%s: create static informers", m.Config.Metadata.DebugName)
		// create informers for each specified object name in each specified namespace
		// This list of informers is static.
		for _, nsName := range nsNames {
			informers, err := m.CreateInformersForNamespace(nsName)
			if err != nil {
				return err
			}
			m.ResourceInformers = append(m.ResourceInformers, informers...)
		}
	}

	staticNamespaces := map[string]bool{}
	for _, nsName := range m.Config.Namespaces() {
		if nsName != "" {
			staticNamespaces[nsName] = true
		}
	}

	if m.Config.NamespaceSelector != nil && m.Config.NamespaceSelector.LabelSelector != nil {
		rlog.Debugf("%s: create ns informer", m.Config.Metadata.DebugName)
		m.NamespaceInformer = NewNamespaceInformer(m.Config)
		err := m.NamespaceInformer.CreateSharedInformer(
			func(nsName string) {
				// add function — check, create and run informers for Ns
				rlog.Infof("NS INFORMER %s: ADD %s", m.Config.Metadata.DebugName, nsName)

				// ignore statically specified namespaces
				if _, ok := staticNamespaces[nsName]; ok {
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
					rlog.Errorf("%s: create informers for ns/%s: %v", m.Config.Metadata.DebugName, nsName, err)
				}

				var ctx context.Context
				ctx, m.cancelForNs[nsName] = context.WithCancel(m.ctx)

				for _, informer := range m.VaryingInformers[nsName] {
					go informer.Run(ctx.Done())
				}
			},
			func(nsName string) {
				// delete function — check, stop and remove informers for Ns
				rlog.Infof("NS INFORMER %s: DELETE %s", m.Config.Metadata.DebugName, nsName)

				// ignore statically specified namespaces
				if _, ok := staticNamespaces[nsName]; ok {
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
	}

	return nil
}

func (m *monitor) CreateInformersForNamespace(namespace string) (informers []ResourceInformer, err error) {
	informers = make([]ResourceInformer, 0)

	objNames := m.Config.Names()
	if len(objNames) == 0 {
		objNames = []string{""}
	}

	for _, objName := range objNames {
		if objName != "" {
			m.Config.AddFieldSelectorRequirement("metadata.name", "=", objName)
		}
		informer := NewResourceInformer(m.Config)
		informer.WithNamespace(namespace)
		informer.WithDebugName(m.Name)

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

	//go m.NamespaceInformer.Run(m.ctx.Done())
	for _, informer := range m.ResourceInformers {
		go informer.Run(m.ctx.Done())
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
