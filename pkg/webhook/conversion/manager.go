package conversion

import (
	"context"
	"os"

	log "github.com/deckhouse/deckhouse/go_lib/log"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/webhook/server"
)

type EventHandlerFn func(cdrName string, request *v1.ConversionRequest) (*Response, error)

// WebhookManager is a public interface to be used from operator.go.
//
// No dynamic configuration for now. The steps are:
// - Init():
//   - Create a router to distinguish conversion requests between CRDs
//   - Create a handler to handle ConversionReview
//   - Create a server that listens for Kubernetes requests
//   - Call AddWebhook() to register a CRD name in conversion bindings in hooks
//
// - Start():
//   - Start server loop.
//   - Update clientConfig in each registered CRD.
type WebhookManager struct {
	KubeClient *klient.Client

	EventHandlerFn EventHandlerFn
	Settings       *WebhookSettings
	Namespace      string

	Server        *server.WebhookServer
	ClientConfigs map[string]*CrdClientConfig
	Handler       *WebhookHandler
}

func NewWebhookManager() *WebhookManager {
	return &WebhookManager{
		ClientConfigs: make(map[string]*CrdClientConfig),
	}
}

// Init creates dependencies
func (m *WebhookManager) Init() error {
	log.Info("Initialize conversion webhooks manager. Load certificates.")

	// settings
	caBundleBytes, err := os.ReadFile(m.Settings.CAPath)
	if err != nil {
		return err
	}
	m.Settings.CABundle = caBundleBytes

	m.Handler = NewWebhookHandler()
	m.Handler.Manager = m

	m.Server = &server.WebhookServer{
		Settings:  &m.Settings.Settings,
		Namespace: m.Namespace,
		Router:    m.Handler.Router,
	}

	return nil
}

// Start webhook server and update spec.conversion in CRDs.
func (m *WebhookManager) Start() error {
	ctx := context.Background()
	log.Info("Start conversion webhooks manager. Load certificates.")

	err := m.Server.Start()
	if err != nil {
		return err
	}

	for _, clientCfg := range m.ClientConfigs {
		err = clientCfg.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *WebhookManager) AddWebhook(webhook *WebhookConfig) {
	if _, ok := m.ClientConfigs[webhook.CrdName]; !ok {
		m.ClientConfigs[webhook.CrdName] = &CrdClientConfig{
			KubeClient:  m.KubeClient,
			CrdName:     webhook.CrdName,
			Path:        "/" + webhook.CrdName,
			Namespace:   m.Namespace,
			ServiceName: m.Settings.ServiceName,
			CABundle:    m.Settings.CABundle,
		}
	}
}
