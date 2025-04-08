package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	klient "github.com/flant/kube-client/client"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

// Monitor holds informers for resources and a namespace informer
type Monitor struct {
	Name       string
	Config     *MonitorConfig
	KubeClient *klient.Client
	// Static list of informers
	ResourceInformers []*ResourceInformer
	// Namespace informer to get new namespaces
	NamespaceInformer *namespaceInformer
	// map of dynamically starting informers
	VaryingInformers sync.Map

	eventCb       func(kemtypes.KubeEvent)
	eventsEnabled bool
	// Index of namespaces statically defined in monitor configuration
	staticNamespaces sync.Map

	cancelForNs sync.Map

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage metric.Storage

	logger *log.Logger
}

func NewMonitor(ctx context.Context, client *klient.Client, mstor metric.Storage, config *MonitorConfig, eventCb func(kemtypes.KubeEvent), logger *log.Logger) *Monitor {
	cctx, cancel := context.WithCancel(ctx)

	return &Monitor{
		ctx:               cctx,
		cancel:            cancel,
		KubeClient:        client,
		metricStorage:     mstor,
		Config:            config,
		eventCb:           eventCb,
		ResourceInformers: make([]*ResourceInformer, 0),
		VaryingInformers:  sync.Map{},
		cancelForNs:       sync.Map{},
		staticNamespaces:  sync.Map{},
		logger:            logger,
	}
}

func (m *Monitor) GetConfig() *MonitorConfig {
	return m.Config
}

// CreateInformers creates all informers and
// a namespace informer if namespace.labelSelector is defined.
// If MonitorConfig.NamespaceSelector.MatchNames is defined, then
// multiple informers are created for each namespace.
// If no NamespaceSelector defined, then one informer is created.
func (m *Monitor) CreateInformers() error {
	logEntry := utils.EnrichLoggerWithLabels(m.logger, m.Config.Metadata.LogLabels).
		With("binding.name", m.Config.Metadata.DebugName)

	if m.Config.Kind == "" && m.Config.ApiVersion == "" {
		logEntry.Debug("Create Informers for Config with empty kind and apiVersion",
			slog.String("value", fmt.Sprintf("%+v", m.Config)))
		return nil
	}

	logEntry.Debug("Create Informers Config: %+v",
		slog.String("value", fmt.Sprintf("%+v", m.Config)))
	nsNames := m.Config.namespaces()
	if len(nsNames) > 0 {
		logEntry.Debug("create static ResourceInformers")

		// create informers for each specified object name in each specified namespace
		// This list of informers is static.
		for _, nsName := range nsNames {
			if nsName != "" {
				m.staticNamespaces.Store(nsName, true)
			}
			informers, err := m.CreateInformersForNamespace(nsName)
			if err != nil {
				return err
			}
			m.ResourceInformers = append(m.ResourceInformers, informers...)
		}
	}

	if m.Config.NamespaceSelector != nil && m.Config.NamespaceSelector.LabelSelector != nil {
		logEntry.Debug("Create NamespaceInformer for namespace.labelSelector")
		m.NamespaceInformer = NewNamespaceInformer(m.ctx, m.KubeClient, m.Config)
		err := m.NamespaceInformer.createSharedInformer(
			func(nsName string) {
				// Added/Modified event: check, create and run informers for Ns
				// ignore event if namespace is already has static ResourceInformers
				if _, ok := m.staticNamespaces.Load(nsName); ok {
					return
				}
				// ignore already started informers
				if _, ok := m.VaryingInformers.Load(nsName); ok {
					return
				}

				logEntry.Info("got ns, create dynamic ResourceInformers", slog.String("name", nsName))

				varyingInformers, err := m.CreateInformersForNamespace(nsName)
				if err != nil {
					logEntry.Error("create ResourceInformers for ns",
						slog.String("name", nsName),
						log.Err(err))
				}
				m.VaryingInformers.Store(nsName, varyingInformers)

				ctx, cancelForNs := context.WithCancel(m.ctx)
				m.cancelForNs.Store(nsName, cancelForNs)

				for _, informer := range varyingInformers {
					informer.withContext(ctx)
					if m.eventsEnabled {
						informer.enableKubeEventCb()
					}
					informer.start()
				}
			},
			func(nsName string) {
				// Delete event: check, stop and remove informers for Ns
				logEntry.Info("deleted ns, stop dynamic ResourceInformers", slog.String("name", nsName))

				// ignore statically specified namespaces
				if _, ok := m.staticNamespaces.Load(nsName); ok {
					return
				}

				// ignore already stopped informers
				if _, ok := m.cancelForNs.Load(nsName); !ok {
					return
				}

				fn, _ := m.cancelForNs.Load(nsName)
				if fn, ok := fn.(context.CancelFunc); ok {
					fn()
				}

				// TODO wait

				m.VaryingInformers.Delete(nsName)
				m.cancelForNs.Delete(nsName)
			},
		)
		if err != nil {
			return fmt.Errorf("create namespace informer: %v", err)
		}
		for nsName := range m.NamespaceInformer.getExistedObjects() {
			logEntry.Info("got ns, create dynamic ResourceInformers", slog.String("name", nsName))

			// ignore event if namespace is already has static ResourceInformers
			if _, ok := m.staticNamespaces.Load(nsName); ok {
				continue
			}

			varyingInformers, err := m.CreateInformersForNamespace(nsName)
			if err != nil {
				logEntry.Error("create ResourceInformers for ns",
					slog.String("name", nsName),
					log.Err(err))
			}
			m.VaryingInformers.Store(nsName, varyingInformers)
		}
	}

	return nil
}

// Snapshot returns all existed objects from all created informers
func (m *Monitor) Snapshot() []kemtypes.ObjectAndFilterResult {
	objects := make([]kemtypes.ObjectAndFilterResult, 0)

	for _, informer := range m.ResourceInformers {
		objects = append(objects, informer.getCachedObjects()...)
	}

	m.VaryingInformers.Range(func(_, value any) bool {
		if value, ok := value.([]*ResourceInformer); ok {
			for _, informer := range value {
				objects = append(objects, informer.getCachedObjects()...)
			}
		}
		return true
	})

	// Sort objects by namespace and name
	sort.Sort(kemtypes.ByNamespaceAndName(objects))

	return objects
}

// EnableKubeEventCb allows execution of event callback for all informers.
// Also executes eventCb for events accumulated during "Synchronization" phase.
func (m *Monitor) EnableKubeEventCb() {
	for _, informer := range m.ResourceInformers {
		informer.enableKubeEventCb()
	}
	// Execute eventCb for events accumulated during "Synchronization" phase.
	m.VaryingInformers.Range(func(_, value any) bool {
		if value, ok := value.([]*ResourceInformer); ok {
			for _, informer := range value {
				informer.enableKubeEventCb()
			}
		}
		return true
	})
	// Enable events for future VaryingInformers.
	m.eventsEnabled = true
}

// CreateInformersForNamespace creates informers bounded to the namespace. If no matchName is specified,
// it is only one informer. If matchName is specified, then multiple informers are created.
//
// If namespace is empty, then informer is bounded to all namespaces.
func (m *Monitor) CreateInformersForNamespace(namespace string) ([]*ResourceInformer, error) {
	informers := make([]*ResourceInformer, 0)
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

		if err := informer.createSharedInformer(); err != nil {
			return nil, err
		}

		informers = append(informers, informer)
	}

	return informers, nil
}

// Start calls Run on all informers.
func (m *Monitor) Start(parentCtx context.Context) {
	m.ctx, m.cancel = context.WithCancel(parentCtx)

	for _, informer := range m.ResourceInformers {
		informer.withContext(m.ctx)
		informer.start()
	}

	m.VaryingInformers.Range(func(key, value any) bool {
		nsName, ok := key.(string)
		if !ok {
			return true
		}
		ctx, cancelForNs := context.WithCancel(m.ctx)
		m.cancelForNs.Store(nsName, cancelForNs)
		if value, ok := value.([]*ResourceInformer); ok {
			for _, informer := range value {
				informer.withContext(ctx)
				informer.start()
			}
		}
		return true
	})

	if m.NamespaceInformer != nil {
		m.NamespaceInformer.withContext(m.ctx)
		m.NamespaceInformer.start()
	}
}

// Stop stops all informers
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// PauseHandleEvents set flags for all informers to ignore incoming events.
// Useful for shutdown without panicking.
// Calling cancel() leads to a race and panicking, see https://github.com/kubernetes/kubernetes/issues/59822
func (m *Monitor) PauseHandleEvents() {
	for _, informer := range m.ResourceInformers {
		informer.pauseHandleEvents()
	}

	m.VaryingInformers.Range(func(_, value any) bool {
		if value, ok := value.([]*ResourceInformer); ok {
			for _, informer := range value {
				informer.pauseHandleEvents()
			}
		}
		return true
	})

	if m.NamespaceInformer != nil {
		m.NamespaceInformer.pauseHandleEvents()
	}
}

func (m *Monitor) SnapshotOperations() (*CachedObjectsInfo /*total*/, *CachedObjectsInfo /*last*/) {
	total := &CachedObjectsInfo{}
	last := &CachedObjectsInfo{}

	for _, informer := range m.ResourceInformers {
		total.add(informer.getCachedObjectsInfo())
		last.add(informer.getCachedObjectsInfoIncrement())
	}

	m.VaryingInformers.Range(func(_, value any) bool {
		if value, ok := value.([]*ResourceInformer); ok {
			for _, informer := range value {
				total.add(informer.getCachedObjectsInfo())
				last.add(informer.getCachedObjectsInfoIncrement())
			}
		}
		return true
	})

	return total, last
}
