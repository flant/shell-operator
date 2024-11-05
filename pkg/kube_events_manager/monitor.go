package kube_events_manager

import (
	"context"
	"fmt"
	"sort"

	"github.com/deckhouse/deckhouse/pkg/log"

	klient "github.com/flant/kube-client/client"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric_storage"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type Monitor interface {
	CreateInformers() error
	Start(context.Context)
	Stop()
	PauseHandleEvents()
	Snapshot() []ObjectAndFilterResult
	EnableKubeEventCb()
	GetConfig() *MonitorConfig
	SnapshotOperations() (total *CachedObjectsInfo, last *CachedObjectsInfo)
}

// Monitor holds informers for resources and a namespace informer
type monitor struct {
	Name       string
	Config     *MonitorConfig
	KubeClient *klient.Client
	// Static list of informers
	ResourceInformers []*resourceInformer
	// Namespace informer to get new namespaces
	NamespaceInformer *namespaceInformer
	// map of dynamically starting informers
	VaryingInformers map[string][]*resourceInformer

	eventCb       func(KubeEvent)
	eventsEnabled bool
	// Index of namespaces statically defined in monitor configuration
	staticNamespaces map[string]bool

	cancelForNs map[string]context.CancelFunc

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage *metric_storage.MetricStorage

	logger *log.Logger
}

func NewMonitor(ctx context.Context, client *klient.Client, mstor *metric_storage.MetricStorage, config *MonitorConfig, eventCb func(KubeEvent), logger *log.Logger) *monitor {
	cctx, cancel := context.WithCancel(ctx)

	return &monitor{
		ctx:               cctx,
		cancel:            cancel,
		KubeClient:        client,
		metricStorage:     mstor,
		Config:            config,
		eventCb:           eventCb,
		ResourceInformers: make([]*resourceInformer, 0),
		VaryingInformers:  make(map[string][]*resourceInformer),
		cancelForNs:       make(map[string]context.CancelFunc),
		staticNamespaces:  make(map[string]bool),
		logger:            logger,
	}
}

func (m *monitor) GetConfig() *MonitorConfig {
	return m.Config
}

// CreateInformers creates all informers and
// a namespace informer if namespace.labelSelector is defined.
// If MonitorConfig.NamespaceSelector.MatchNames is defined, then
// multiple informers are created for each namespace.
// If no NamespaceSelector defined, then one informer is created.
func (m *monitor) CreateInformers() error {
	logEntry := utils.EnrichLoggerWithLabels(m.logger, m.Config.Metadata.LogLabels).
		With("binding.name", m.Config.Metadata.DebugName)

	if m.Config.Kind == "" && m.Config.ApiVersion == "" {
		logEntry.Debugf("Create Informers for Config with empty kind and apiVersion: %+v", m.Config)
		return nil
	}

	logEntry.Debugf("Create Informers Config: %+v", m.Config)
	nsNames := m.Config.namespaces()
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
		m.NamespaceInformer = NewNamespaceInformer(m.ctx, m.KubeClient, m.Config)
		err := m.NamespaceInformer.createSharedInformer(
			func(nsName string) {
				// Added/Modified event: check, create and run informers for Ns
				// ignore event if namespace is already has static ResourceInformers
				if _, ok := m.staticNamespaces[nsName]; ok {
					return
				}
				// ignore already started informers
				_, ok := m.VaryingInformers[nsName]
				if ok {
					return
				}

				logEntry.Infof("got ns/%s, create dynamic ResourceInformers", nsName)

				var err error
				m.VaryingInformers[nsName], err = m.CreateInformersForNamespace(nsName)
				if err != nil {
					logEntry.Errorf("create ResourceInformers for ns/%s: %v", nsName, err)
				}

				var ctx context.Context
				ctx, m.cancelForNs[nsName] = context.WithCancel(m.ctx)

				for _, informer := range m.VaryingInformers[nsName] {
					informer.withContext(ctx)
					if m.eventsEnabled {
						informer.enableKubeEventCb()
					}
					informer.start()
				}
			},
			func(nsName string) {
				// Delete event: check, stop and remove informers for Ns
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
		for nsName := range m.NamespaceInformer.getExistedObjects() {
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

// Snapshot returns all existed objects from all created informers
func (m *monitor) Snapshot() []ObjectAndFilterResult {
	objects := make([]ObjectAndFilterResult, 0)

	for _, informer := range m.ResourceInformers {
		objects = append(objects, informer.getCachedObjects()...)
	}

	for nsName := range m.VaryingInformers {
		for _, informer := range m.VaryingInformers[nsName] {
			objects = append(objects, informer.getCachedObjects()...)
		}
	}

	// Sort objects by namespace and name
	sort.Sort(ByNamespaceAndName(objects))

	return objects
}

// EnableKubeEventCb allows execution of event callback for all informers.
// Also executes eventCb for events accumulated during "Synchronization" phase.
func (m *monitor) EnableKubeEventCb() {
	for _, informer := range m.ResourceInformers {
		informer.enableKubeEventCb()
	}
	for nsName := range m.VaryingInformers {
		for _, informer := range m.VaryingInformers[nsName] {
			informer.enableKubeEventCb()
		}
	}
	// Enable events for future VaryingInformers.
	m.eventsEnabled = true
}

// CreateInformersForNamespace creates informers bounded to the namespace. If no matchName is specified,
// it is only one informer. If matchName is specified, then multiple informers are created.
//
// If namespace is empty, then informer is bounded to all namespaces.
func (m *monitor) CreateInformersForNamespace(namespace string) (informers []*resourceInformer, err error) {
	informers = make([]*resourceInformer, 0)
	cfg := &resourceInformerConfig{
		client:  m.KubeClient,
		mstor:   m.metricStorage,
		eventCb: m.eventCb,
		monitor: m.Config,
		logger:  m.logger.Named("resource-informer"),
	}

	objNames := []string{""}

	if len(m.Config.names()) > 0 {
		objNames = m.Config.names()
	}

	for _, objName := range objNames {
		informer := newResourceInformer(namespace, objName, cfg)

		err := informer.createSharedInformer()
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
		informer.withContext(m.ctx)
		informer.start()
	}

	for nsName := range m.VaryingInformers {
		var ctx context.Context
		ctx, m.cancelForNs[nsName] = context.WithCancel(m.ctx)
		for _, informer := range m.VaryingInformers[nsName] {
			informer.withContext(ctx)
			informer.start()
		}
	}

	if m.NamespaceInformer != nil {
		m.NamespaceInformer.withContext(m.ctx)
		m.NamespaceInformer.start()
	}
}

// Stop stops all informers
func (m *monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// PauseHandleEvents set flags for all informers to ignore incoming events.
// Useful for shutdown without panicking.
// Calling cancel() leads to a race and panicking, see https://github.com/kubernetes/kubernetes/issues/59822
func (m *monitor) PauseHandleEvents() {
	for _, informer := range m.ResourceInformers {
		informer.pauseHandleEvents()
	}

	for _, informers := range m.VaryingInformers {
		for _, informer := range informers {
			informer.pauseHandleEvents()
		}
	}

	if m.NamespaceInformer != nil {
		m.NamespaceInformer.pauseHandleEvents()
	}
}

func (m *monitor) SnapshotOperations() (total *CachedObjectsInfo, last *CachedObjectsInfo) {
	total = &CachedObjectsInfo{}
	last = &CachedObjectsInfo{}

	for _, informer := range m.ResourceInformers {
		total.add(informer.getCachedObjectsInfo())
		last.add(informer.getCachedObjectsInfoIncrement())
	}

	for nsName := range m.VaryingInformers {
		for _, informer := range m.VaryingInformers[nsName] {
			total.add(informer.getCachedObjectsInfo())
			last.add(informer.getCachedObjectsInfoIncrement())
		}
	}

	return total, last
}
