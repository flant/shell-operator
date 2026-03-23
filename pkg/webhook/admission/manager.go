package admission

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/pkg/log"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

// DefaultConfigurationId is a ConfigurationId for ValidatingWebhookConfiguration/MutatingWebhookConfiguration
// without suffix.
const DefaultConfigurationId = "hooks"

// WebhookManager is a public interface to be used from operator.go.
//
// No dynamic configuration for now. The steps are:
//   - Init manager
//   - Call AddWebhook for every binding in hooks
//   - Start() to run server and create ValidatingWebhookConfiguration/MutatingWebhookConfiguration
type WebhookManager struct {
	KubeClient *klient.Client

	Settings  *WebhookSettings
	Namespace string

	DefaultConfigurationId string

	Server              *server.WebhookServer
	ValidatingResources map[string]*ValidatingWebhookResource
	MutatingResources   map[string]*MutatingWebhookResource
	Handler             *WebhookHandler

	Logger *log.Logger
}

type Option func(manager *WebhookManager)

func WithLogger(logger *log.Logger) Option {
	return func(manager *WebhookManager) {
		manager.Logger = logger
	}
}

func NewWebhookManager(kubeClient *klient.Client, opts ...Option) *WebhookManager {
	manager := &WebhookManager{
		ValidatingResources: make(map[string]*ValidatingWebhookResource),
		MutatingResources:   make(map[string]*MutatingWebhookResource),
		KubeClient:          kubeClient,
		Logger:              log.NewLogger().Named("admission-webhook-manager"),
	}

	for _, opt := range opts {
		opt(manager)
	}

	return manager
}

func (m *WebhookManager) WithAdmissionEventHandler(handler EventHandlerFn) {
	if m.Handler == nil {
		m.Handler = &WebhookHandler{
			Handler: handler,
		}
	} else {
		m.Handler.Handler = handler
	}
}

// Init creates dependencies
func (m *WebhookManager) Init() error {
	m.Logger.Info("Initialize admission webhooks manager. Load certificates.")

	if m.DefaultConfigurationId == "" {
		m.DefaultConfigurationId = DefaultConfigurationId
	}
	// settings
	caBundleBytes, err := os.ReadFile(m.Settings.CAPath)
	if err != nil {
		return fmt.Errorf("read CA bundle: %w", err)
	}
	m.Settings.CABundle = caBundleBytes

	m.Handler = NewWebhookHandler(m.Logger)

	m.Server = server.NewWebhookServer(&m.Settings.Settings, m.Namespace, m.Handler.Router, m.Logger)

	return nil
}

func (m *WebhookManager) AddValidatingWebhook(config *ValidatingWebhookConfig) {
	confId := config.Metadata.ConfigurationId
	if confId == "" {
		confId = m.DefaultConfigurationId
	}
	r, ok := m.ValidatingResources[confId]
	if !ok {
		r = NewValidatingWebhookResource(
			WebhookResourceOptions{
				m.KubeClient,
				m.Namespace,
				m.Settings.ConfigurationName + "-" + confId,
				m.Settings.ServiceName,
				m.Settings.CABundle,
			},
			m.Logger,
		)
		m.ValidatingResources[confId] = r
	}

	r.Set(config)
}

func (m *WebhookManager) AddMutatingWebhook(config *MutatingWebhookConfig) {
	confId := config.Metadata.ConfigurationId
	if confId == "" {
		confId = m.DefaultConfigurationId
	}
	r, ok := m.MutatingResources[confId]
	if !ok {
		r = NewMutatingWebhookResource(
			WebhookResourceOptions{
				m.KubeClient,
				m.Namespace,
				m.Settings.ConfigurationName + "-" + confId,
				m.Settings.ServiceName,
				m.Settings.CABundle,
			},
			m.Logger,
		)
		m.MutatingResources[confId] = r
	}
	r.Set(config)
}

func (m *WebhookManager) Start() error {
	err := m.Server.Start()
	if err != nil {
		return fmt.Errorf("start webhook server: %w", err)
	}

	for _, r := range m.ValidatingResources {
		err = r.Register()
		if err != nil {
			return fmt.Errorf("register validating webhook: %w", err)
		}
	}

	for _, r := range m.MutatingResources {
		err = r.Register()
		if err != nil {
			return fmt.Errorf("register mutating webhook: %w", err)
		}
	}

	return nil
}
