package admission

import (
	"context"
	"log/slog"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg"
)

type WebhookResourceOptions struct {
	KubeClient        *klient.Client
	Namespace         string
	ConfigurationName string
	ServiceName       string
	CABundle          []byte
}

type ValidatingWebhookResource struct {
	hooks map[string]*ValidatingWebhookConfig
	opts  WebhookResourceOptions
}

func NewValidatingWebhookResource(opts WebhookResourceOptions) *ValidatingWebhookResource {
	return &ValidatingWebhookResource{
		hooks: make(map[string]*ValidatingWebhookConfig),
		opts:  opts,
	}
}

func (w *ValidatingWebhookResource) Set(whc *ValidatingWebhookConfig) {
	w.hooks[whc.Metadata.WebhookId] = whc
}

func (w *ValidatingWebhookResource) Get(id string) *ValidatingWebhookConfig {
	return w.hooks[id]
}

func (w *ValidatingWebhookResource) Register() error {
	configuration := &v1.ValidatingWebhookConfiguration{
		Webhooks: []v1.ValidatingWebhook{},
	}
	configuration.Name = w.opts.ConfigurationName

	for _, webhook := range w.hooks {
		equivalent := v1.Equivalent
		webhook.MatchPolicy = &equivalent
		webhook.AdmissionReviewVersions = []string{"v1", "v1beta1"}
		webhook.ClientConfig = v1.WebhookClientConfig{
			Service: &v1.ServiceReference{
				Namespace: w.opts.Namespace,
				Name:      w.opts.ServiceName,
				Path:      createWebhookPath(IWebhookConfig(webhook)),
			},
			CABundle: w.opts.CABundle,
		}

		log.Info("Add path to config",
			slog.String("path", *webhook.ClientConfig.Service.Path),
			slog.String("configurationName", w.opts.ConfigurationName))

		configuration.Webhooks = append(configuration.Webhooks, *webhook.ValidatingWebhook)
	}

	return w.submit(configuration)
}

func (w *ValidatingWebhookResource) Unregister() error {
	return w.opts.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Delete(context.TODO(), w.opts.ConfigurationName, metav1.DeleteOptions{})
}

func createWebhookPath(webhook IWebhookConfig) *string {
	s := new(strings.Builder)

	s.WriteString("/")
	if webhook.GetMeta().ConfigurationId == "" {
		s.WriteString(DefaultConfigurationId)
	} else {
		s.WriteString(webhook.GetMeta().ConfigurationId)
	}

	s.WriteString("/")
	s.WriteString(webhook.GetMeta().WebhookId)

	res := s.String()
	return &res
}

func (w *ValidatingWebhookResource) submit(conf *v1.ValidatingWebhookConfiguration) error {
	client := w.opts.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations()

	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + conf.Name,
	}
	list, err := client.List(context.TODO(), listOpts)
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		_, err = client.Create(context.TODO(), conf, pkg.DefaultCreateOptions())
		if err != nil {
			log.Error("Create ValidatingWebhookConfiguration",
				slog.String("name", conf.Name),
				log.Err(err))
		}
	} else {
		newConf := list.Items[0]
		newConf.Webhooks = conf.Webhooks
		_, err = client.Update(context.TODO(), &newConf, pkg.DefaultUpdateOptions())
		if err != nil {
			log.Error("Replace ValidatingWebhookConfiguration",
				slog.String("name", conf.Name),
				log.Err(err))
		}
	}
	return nil
}

type MutatingWebhookResource struct {
	hooks map[string]*MutatingWebhookConfig
	opts  WebhookResourceOptions
}

func NewMutatingWebhookResource(opts WebhookResourceOptions) *MutatingWebhookResource {
	return &MutatingWebhookResource{
		hooks: make(map[string]*MutatingWebhookConfig),
		opts:  opts,
	}
}

func (w *MutatingWebhookResource) Set(whc *MutatingWebhookConfig) {
	w.hooks[whc.Metadata.WebhookId] = whc
}

func (w *MutatingWebhookResource) Get(id string) *MutatingWebhookConfig {
	return w.hooks[id]
}

func (w *MutatingWebhookResource) Register() error {
	configuration := &v1.MutatingWebhookConfiguration{
		Webhooks: []v1.MutatingWebhook{},
	}
	configuration.Name = w.opts.ConfigurationName

	for _, webhook := range w.hooks {
		equivalent := v1.Equivalent
		webhook.MatchPolicy = &equivalent
		webhook.AdmissionReviewVersions = []string{"v1", "v1beta1"}
		webhook.ClientConfig = v1.WebhookClientConfig{
			Service: &v1.ServiceReference{
				Namespace: w.opts.Namespace,
				Name:      w.opts.ServiceName,
				Path:      createWebhookPath(IWebhookConfig(webhook)),
			},
			CABundle: w.opts.CABundle,
		}

		log.Info("Add path to config",
			slog.String("path", *webhook.ClientConfig.Service.Path),
			slog.String("configurationName", w.opts.ConfigurationName))

		configuration.Webhooks = append(configuration.Webhooks, *webhook.MutatingWebhook)
	}

	return w.submit(configuration)
}

func (w *MutatingWebhookResource) Unregister() error {
	return w.opts.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().
		Delete(context.TODO(), w.opts.ConfigurationName, metav1.DeleteOptions{})
}

func (w *MutatingWebhookResource) submit(conf *v1.MutatingWebhookConfiguration) error {
	client := w.opts.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations()

	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + conf.Name,
	}
	list, err := client.List(context.TODO(), listOpts)
	if err != nil {
		return err
	}
	if len(list.Items) == 0 {
		_, err = client.Create(context.TODO(), conf, pkg.DefaultCreateOptions())
		if err != nil {
			log.Error("Create MutatingWebhookConfiguration",
				slog.String("name", conf.Name),
				log.Err(err))
		}
	} else {
		newConf := list.Items[0]
		newConf.Webhooks = conf.Webhooks
		_, err = client.Update(context.TODO(), &newConf, pkg.DefaultUpdateOptions())
		if err != nil {
			log.Error("Replace MutatingWebhookConfiguration",
				slog.String("name", conf.Name),
				log.Err(err))
		}
	}
	return nil
}
