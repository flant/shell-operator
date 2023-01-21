package admission

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"

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
//   - Call AddWEbhook for every binding in hooks
//   - Start() to run server and create ValidatingWebhookConfiguration/MutatingWebhookConfiguration
type WebhookManager struct {
	KubeClient klient.Client

	Settings  *WebhookSettings
	Namespace string

	DefaultConfigurationId string

	Server              *server.WebhookServer
	ValidatingResources map[string]*ValidatingWebhookResource
	MutatingResources   map[string]*MutatingWebhookResource
	Handler             *WebhookHandler
}

func NewWebhookManager() *WebhookManager {
	return &WebhookManager{
		ValidatingResources: make(map[string]*ValidatingWebhookResource),
		MutatingResources:   make(map[string]*MutatingWebhookResource),
	}
}

func (m *WebhookManager) WithKubeClient(kubeClient klient.Client) {
	m.KubeClient = kubeClient
}

func (m *WebhookManager) WithAdmissionEventHandler(handler AdmissionEventHandlerFn) {
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
	log.Info("Initialize validating webhooks manager. Load certificates.")

	if m.DefaultConfigurationId == "" {
		m.DefaultConfigurationId = DefaultConfigurationId
	}
	// settings
	caBundleBytes, err := ioutil.ReadFile(m.Settings.CAPath)
	if err != nil {
		return err
	}
	m.Settings.CABundle = caBundleBytes

	m.Handler = NewWebhookHandler()

	m.Server = &server.WebhookServer{
		Settings:  &m.Settings.Settings,
		Namespace: m.Namespace,
		Router:    m.Handler.Router,
	}

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
		)
		m.MutatingResources[confId] = r
	}
	r.Set(config)
}

func (m *WebhookManager) Start() error {
	err := m.Server.Start()
	if err != nil {
		return err
	}

	for _, r := range m.ValidatingResources {
		err = r.Register()
		if err != nil {
			return err
		}
	}

	for _, r := range m.MutatingResources {
		err = r.Register()
		if err != nil {
			return err
		}
	}

	return nil
}
