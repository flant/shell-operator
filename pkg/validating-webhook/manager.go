package validating_webhook

import (
	"io/ioutil"

	v1 "k8s.io/api/admission/v1"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
)

type ReviewHandlerFn func(review *v1.AdmissionReview) (*ReviewResponse, error)

// WebhookManager is a public interface.
type WebhookManager struct {
	KubeClient kube.KubernetesClient

	ReviewHandlerFn ReviewHandlerFn

	Server   *WebhookServer
	Resource *WebhookResource
	Handler  *WebhookHandler
}

func (m *WebhookManager) WithKubeClient(kubeClient kube.KubernetesClient) {
	m.KubeClient = kubeClient
}

func (m *WebhookManager) WithReviewHandler(handler ReviewHandlerFn) {
	m.ReviewHandlerFn = handler
}

func (m *WebhookManager) Start() error {
	// settings
	caBundleBytes, err := ioutil.ReadFile(app.ValidatingWebhookSettings.ClusterCAPath)
	if err != nil {
		return err
	}
	app.ValidatingWebhookSettings.ClusterCABundle = caBundleBytes

	m.Handler = NewWebhookHandler()
	m.Handler.Manager = m

	m.Server = &WebhookServer{
		Router: m.Handler.Router,
	}
	err = m.Server.Start()
	if err != nil {
		return err
	}

	// Recreate resource.
	m.Resource = &WebhookResource{
		KubeClient: m.KubeClient,
	}

	//err = m.Resource.CreateWebhook()
	//if err != nil {
	//	return err
	//}

	return nil
}
