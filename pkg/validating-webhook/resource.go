package validating_webhook

import (
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebhookResource struct {
	KubeClient kube.KubernetesClient
}

func (w *WebhookResource) CreateWebhook() error {
	fail := v1.Fail
	equivalent := v1.Equivalent
	timeoutSeconds := int32(10)

	webhook := &v1.ValidatingWebhookConfiguration{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Webhooks: []v1.ValidatingWebhook{
			{
				Name: "shell-operator-validating",
				ClientConfig: v1.WebhookClientConfig{
					Service: &v1.ServiceReference{
						Namespace: app.Namespace,
						Name:      app.ValidatingWebhookSettings.ServiceName,
					},
					CABundle: app.ValidatingWebhookSettings.ClusterCABundle,
				},
				MatchPolicy:             &equivalent,
				SideEffects:             nil,
				AdmissionReviewVersions: []string{"v1"},
				// These are from hook configuration:
				FailurePolicy:     &fail,
				NamespaceSelector: nil,
				ObjectSelector:    nil,
				TimeoutSeconds:    &timeoutSeconds,
				Rules:             nil,
			},
		},
	}

	_, err := w.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(webhook)
	if err != nil {
		return err
	}

	return nil
}
