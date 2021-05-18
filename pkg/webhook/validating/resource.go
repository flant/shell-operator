package validating

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
)

type WebhookResource struct {
	KubeClient        kube.KubernetesClient
	Webhooks          map[string]*ValidatingWebhookConfig
	Namespace         string
	ConfigurationName string
	ServiceName       string
	CABundle          []byte
}

func NewWebhookResource() *WebhookResource {
	return &WebhookResource{
		Webhooks: make(map[string]*ValidatingWebhookConfig),
	}
}

func (w *WebhookResource) AddWebhook(config *ValidatingWebhookConfig) {
	w.Webhooks[config.Metadata.WebhookId] = config
}

func (w *WebhookResource) CreateConfiguration() error {
	equivalent := v1.Equivalent

	configuration := &v1.ValidatingWebhookConfiguration{
		Webhooks: []v1.ValidatingWebhook{},
	}
	configuration.Name = w.ConfigurationName

	for _, webhook := range w.Webhooks {
		webhook.MatchPolicy = &equivalent
		webhook.AdmissionReviewVersions = []string{"v1", "v1beta1"}
		webhook.ClientConfig = v1.WebhookClientConfig{
			Service: &v1.ServiceReference{
				Namespace: w.Namespace,
				Name:      w.ServiceName,
				Path:      w.CreateWebhookPath(webhook),
			},
			CABundle: w.CABundle,
		}

		log.Infof("Add '%s' path to '%s'", *webhook.ClientConfig.Service.Path, w.ConfigurationName)

		configuration.Webhooks = append(configuration.Webhooks, *webhook.ValidatingWebhook)
	}

	return w.CreateOrUpdateConfiguration(configuration)
}

func (w *WebhookResource) UpdateConfiguration() error {
	return nil
}

func (w *WebhookResource) DeleteConfiguration() error {
	return w.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Delete(context.TODO(), w.ConfigurationName, metav1.DeleteOptions{})
}

func (w *WebhookResource) CreateWebhookPath(webhook *ValidatingWebhookConfig) *string {
	s := new(strings.Builder)

	s.WriteString("/")
	if webhook.Metadata.ConfigurationId == "" {
		s.WriteString(DefaultConfigurationId)
	} else {
		s.WriteString(webhook.Metadata.ConfigurationId)
	}

	s.WriteString("/")
	s.WriteString(webhook.Metadata.WebhookId)

	res := s.String()
	return &res
}

func (w *WebhookResource) CreateOrUpdateConfiguration(conf *v1.ValidatingWebhookConfiguration) error {
	client := w.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations()

	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + conf.Name,
	}
	list, err := client.List(context.TODO(), listOpts)
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		_, err = client.Create(context.TODO(), conf, metav1.CreateOptions{})
		if err != nil {
			log.Errorf("Create ValidatingWebhookConfiguration/%s: %v", conf.Name, err)
		}
	} else {
		newConf := list.Items[0]
		newConf.Webhooks = conf.Webhooks
		_, err = client.Update(context.TODO(), &newConf, metav1.UpdateOptions{})
		if err != nil {
			log.Errorf("Replace ValidatingWebhookConfiguration/%s: %v", conf.Name, err)
		}
	}
	return nil
}
