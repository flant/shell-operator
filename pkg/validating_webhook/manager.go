package validating_webhook

import (
	log "github.com/sirupsen/logrus"
	"io/ioutil"

	. "github.com/flant/shell-operator/pkg/validating_webhook/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
)

type ValidatingEventHandlerFn func(event ValidatingEvent) (*ValidatingResponse, error)

// DefaultConfigurationId is a ConfigurationId for ValidatingWebhookConfiguration
// without suffix.
const DefaultConfigurationId = "hooks"

// WebhookManager is a public interface to be used from operator.go.
//
// No dynamic configuration for now. The steps are:
//   - Init manager
//   - Call AddWEbhook for every binding in hooks
//   - Start() to run server and create ValidatingWebhookConfiguration
type WebhookManager struct {
	KubeClient kube.KubernetesClient

	ValidatingEventHandlerFn ValidatingEventHandlerFn

	Namespace         string
	ConfigurationName string
	ServiceName       string

	CABundle               []byte
	DefaultConfigurationId string

	Server    *WebhookServer
	Resources map[string]*WebhookResource
	Handler   *WebhookHandler
}

func NewWebhookManager() *WebhookManager {
	return &WebhookManager{
		Resources: make(map[string]*WebhookResource),
	}
}

func (m *WebhookManager) WithKubeClient(kubeClient kube.KubernetesClient) {
	m.KubeClient = kubeClient
}

func (m *WebhookManager) WithValidatingEventHandler(handler ValidatingEventHandlerFn) {
	m.ValidatingEventHandlerFn = handler
}

// Init creates dependencies
func (m *WebhookManager) Init() error {
	log.Info("Initialize validating webhooks manager. Load certificates.")

	if m.DefaultConfigurationId == "" {
		m.DefaultConfigurationId = DefaultConfigurationId
	}
	// settings
	caBundleBytes, err := ioutil.ReadFile(app.ValidatingWebhookSettings.ClusterCAPath)
	if err != nil {
		return err
	}
	m.CABundle = caBundleBytes

	m.Handler = NewWebhookHandler()
	m.Handler.Manager = m

	m.Server = &WebhookServer{
		Router: m.Handler.Router,
	}

	m.Resources = make(map[string]*WebhookResource)
	r := NewWebhookResource()
	r.KubeClient = m.KubeClient
	r.Namespace = m.Namespace
	r.ConfigurationName = m.ConfigurationName
	r.ServiceName = m.ServiceName
	r.CABundle = m.CABundle
	m.Resources[m.DefaultConfigurationId] = r

	return nil
}

func (m *WebhookManager) AddWebhook(config *ValidatingWebhookConfig) {
	confId := config.Metadata.ConfigurationId
	if confId == "" {
		confId = m.DefaultConfigurationId
	}
	r, ok := m.Resources[confId]
	if !ok {
		r = NewWebhookResource()
		r.KubeClient = m.KubeClient
		r.Namespace = m.Namespace
		r.ConfigurationName = m.ConfigurationName + "-" + confId
		r.ServiceName = m.ServiceName
		r.CABundle = m.CABundle
		m.Resources[confId] = r
	}
	r.AddWebhook(config)
}

func (m *WebhookManager) Start() error {
	err := m.Server.Start()
	if err != nil {
		return err
	}

	for _, r := range m.Resources {
		err = r.CreateConfiguration()
		if err != nil {
			return err
		}
	}

	return nil
}
