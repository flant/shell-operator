package admission

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/utils/retry"
)

const (
	// submitMaxRetries is the number of retry attempts for webhook configuration
	// registration against the API server. Brief API server unavailability (e.g.
	// during rolling restarts or leader election) should not cause a fatal exit.
	//
	// This retry is intentionally scoped to API-server registration calls only.
	// Do not wrap full webhook-manager bootstrap with another retry layer: server
	// Start() is not idempotent and may leave running listeners/goroutines.
	submitMaxRetries = 5
	// submitInitialBackoff is the initial delay between retries. It doubles on
	// each consecutive failure up to submitMaxBackoff.
	submitInitialBackoff = 2 * time.Second
	submitMaxBackoff     = 15 * time.Second
)

var submitRetryConfig = retry.Config{
	MaxRetries:     submitMaxRetries,
	InitialBackoff: submitInitialBackoff,
	MaxBackoff:     submitMaxBackoff,
}

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

	logger *log.Logger
}

func NewValidatingWebhookResource(opts WebhookResourceOptions, logger *log.Logger) *ValidatingWebhookResource {
	return &ValidatingWebhookResource{
		hooks:  make(map[string]*ValidatingWebhookConfig),
		opts:   opts,
		logger: logger,
	}
}

func (w *ValidatingWebhookResource) Set(whc *ValidatingWebhookConfig) {
	w.hooks[whc.Metadata.WebhookId] = whc
}

func (w *ValidatingWebhookResource) Get(id string) *ValidatingWebhookConfig {
	return w.hooks[id]
}

func (w *ValidatingWebhookResource) Register(ctx context.Context) error {
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

		w.logger.Info("Add path to config",
			slog.String(pkg.LogKeyPath, *webhook.ClientConfig.Service.Path),
			slog.String(pkg.LogKeyConfigurationName, w.opts.ConfigurationName))

		configuration.Webhooks = append(configuration.Webhooks, *webhook.ValidatingWebhook)
	}

	return w.submit(ctx, configuration)
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

func (w *ValidatingWebhookResource) submit(ctx context.Context, conf *v1.ValidatingWebhookConfiguration) error {
	client := w.opts.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + conf.Name,
	}

	return retry.WithBackoff(ctx, submitRetryConfig, w.logger.With(slog.String(pkg.LogKeyName, conf.Name)),
		"ValidatingWebhookConfiguration/submit", func() error {
			list, err := client.List(ctx, listOpts)
			if err != nil {
				return fmt.Errorf("list ValidatingWebhookConfiguration: %w", err)
			}
			if len(list.Items) == 0 {
				if _, err := client.Create(ctx, conf, pkg.DefaultCreateOptions()); err != nil {
					return fmt.Errorf("create ValidatingWebhookConfiguration: %w", err)
				}
			} else {
				newConf := list.Items[0]
				newConf.Webhooks = conf.Webhooks
				if _, err := client.Update(ctx, &newConf, pkg.DefaultUpdateOptions()); err != nil {
					return fmt.Errorf("update ValidatingWebhookConfiguration: %w", err)
				}
			}
			return nil
		})
}

type MutatingWebhookResource struct {
	hooks map[string]*MutatingWebhookConfig
	opts  WebhookResourceOptions

	logger *log.Logger
}

func NewMutatingWebhookResource(opts WebhookResourceOptions, logger *log.Logger) *MutatingWebhookResource {
	return &MutatingWebhookResource{
		hooks:  make(map[string]*MutatingWebhookConfig),
		opts:   opts,
		logger: logger,
	}
}

func (w *MutatingWebhookResource) Set(whc *MutatingWebhookConfig) {
	w.hooks[whc.Metadata.WebhookId] = whc
}

func (w *MutatingWebhookResource) Get(id string) *MutatingWebhookConfig {
	return w.hooks[id]
}

func (w *MutatingWebhookResource) Register(ctx context.Context) error {
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

		w.logger.Info("Add path to config",
			slog.String(pkg.LogKeyPath, *webhook.ClientConfig.Service.Path),
			slog.String(pkg.LogKeyConfigurationName, w.opts.ConfigurationName))

		configuration.Webhooks = append(configuration.Webhooks, *webhook.MutatingWebhook)
	}

	return w.submit(ctx, configuration)
}

func (w *MutatingWebhookResource) Unregister() error {
	return w.opts.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().
		Delete(context.TODO(), w.opts.ConfigurationName, metav1.DeleteOptions{})
}

func (w *MutatingWebhookResource) submit(ctx context.Context, conf *v1.MutatingWebhookConfiguration) error {
	client := w.opts.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations()
	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + conf.Name,
	}

	return retry.WithBackoff(ctx, submitRetryConfig, w.logger.With(slog.String(pkg.LogKeyName, conf.Name)),
		"MutatingWebhookConfiguration/submit", func() error {
			list, err := client.List(ctx, listOpts)
			if err != nil {
				return fmt.Errorf("list MutatingWebhookConfiguration: %w", err)
			}
			if len(list.Items) == 0 {
				if _, err := client.Create(ctx, conf, pkg.DefaultCreateOptions()); err != nil {
					return fmt.Errorf("create MutatingWebhookConfiguration: %w", err)
				}
			} else {
				newConf := list.Items[0]
				newConf.Webhooks = conf.Webhooks
				if _, err := client.Update(ctx, &newConf, pkg.DefaultUpdateOptions()); err != nil {
					return fmt.Errorf("update MutatingWebhookConfiguration: %w", err)
				}
			}
			return nil
		})
}
