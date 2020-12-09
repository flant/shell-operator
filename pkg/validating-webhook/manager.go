package validating_webhook

import (
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	"io/ioutil"
)

type WebhookManager struct {
	KubeClient kube.KubernetesClient

	Server   *WebhookServer
	Resource *WebhookResource
	Handler  *WebhookHandler
}

func (m *WebhookManager) Start() error {
	// settings
	caBundleBytes, err := ioutil.ReadFile(app.ValidatingWebhookSettings.ClusterCAPath)
	if err != nil {
		return err
	}
	app.ValidatingWebhookSettings.ClusterCABundle = caBundleBytes

	m.Handler = NewWebhookHandler()

	m.Server = &WebhookServer{
		Router: m.Handler.Router,
	}
	err = m.Server.Start()
	if err != nil {
		return err
	}

	// create resource...
	m.Resource = &WebhookResource{
		KubeClient: m.KubeClient,
	}

	//err = m.Resource.CreateWebhook()
	//if err != nil {
	//	return err
	//}

	return nil
}
