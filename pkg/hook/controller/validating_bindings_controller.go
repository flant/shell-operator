package controller

import (
	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/webhook/admission/types"

	"github.com/flant/shell-operator/pkg/webhook/admission"
)

// A link between a hook and a kube monitor
type ValidatingBindingToWebhookLink struct {
	BindingName     string
	ConfigurationId string
	WebhookId       string
	// Useful fields to create a BindingContext
	IncludeSnapshots []string
	Group            string
}

// ScheduleBindingsController handles schedule bindings for one hook.
type ValidatingBindingsController interface {
	WithValidatingBindings([]ValidatingConfig)
	WithWebhookManager(*admission.WebhookManager)
	EnableValidatingBindings()
	DisableValidatingBindings()
	CanHandleEvent(event AdmissionEvent) bool
	HandleEvent(event AdmissionEvent) BindingExecutionInfo
}

type validatingBindingsController struct {
	// Controller holds validating bindings from one hook. Hook always belongs to one configurationId.
	ConfigurationId string
	// WebhookId -> link
	ValidatingLinks map[string]*ValidatingBindingToWebhookLink

	ValidatingBindings []ValidatingConfig
	MutatingBindings   []MutatingConfig

	webhookManager *admission.WebhookManager
}

var _ ValidatingBindingsController = &validatingBindingsController{}

// NewKubernetesHooksController returns an implementation of KubernetesHooksController
var NewValidatingBindingsController = func() *validatingBindingsController {
	return &validatingBindingsController{
		ValidatingLinks: make(map[string]*ValidatingBindingToWebhookLink),
	}
}

func (c *validatingBindingsController) WithValidatingBindings(bindings []ValidatingConfig) {
	c.ValidatingBindings = bindings
}

func (c *validatingBindingsController) WithMutatingBindings(bindings []MutatingConfig) {
	c.MutatingBindings = bindings
}

func (c *validatingBindingsController) WithWebhookManager(mgr *admission.WebhookManager) {
	c.webhookManager = mgr
}

func (c *validatingBindingsController) EnableValidatingBindings() {
	confId := ""
	for _, config := range c.ValidatingBindings {
		if config.Webhook.Metadata.ConfigurationId == "" && confId == "" {
			continue
		}
		if config.Webhook.Metadata.ConfigurationId != "" && confId == "" {
			confId = config.Webhook.Metadata.ConfigurationId
			continue
		}
		if config.Webhook.Metadata.ConfigurationId != confId {
			log.Errorf("Possible bug!!! kubernetesValidating has non-unique configurationIds: '%s' '%s'", config.Webhook.Metadata.ConfigurationId, confId)
		}
	}
	c.ConfigurationId = confId

	for _, config := range c.ValidatingBindings {
		c.ValidatingLinks[config.Webhook.Metadata.WebhookId] = &ValidatingBindingToWebhookLink{
			BindingName:      config.BindingName,
			ConfigurationId:  c.ConfigurationId,
			WebhookId:        config.Webhook.Metadata.WebhookId,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			Group:            config.Group,
		}
		c.webhookManager.AddValidatingWebhook(config.Webhook)
	}
}

func (c *validatingBindingsController) DisableValidatingBindings() {
	// TODO dynamic enable/disable validating webhooks.
}

func (c *validatingBindingsController) CanHandleEvent(event AdmissionEvent) bool {
	if c.ConfigurationId != event.ConfigurationId {
		return false
	}
	_, has := c.ValidatingLinks[event.WebhookId]
	return has
}

func (c *validatingBindingsController) HandleEvent(event AdmissionEvent) BindingExecutionInfo {
	if c.ConfigurationId != event.ConfigurationId {
		log.Errorf("Possible bug!!! Unknown validating event: no binding for configurationId '%s' (webhookId '%s')", event.ConfigurationId, event.WebhookId)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}

	link, hasKey := c.ValidatingLinks[event.WebhookId]
	if !hasKey {
		log.Errorf("Possible bug!!! Unknown validating event: no binding for configurationId '%s', webhookId '%s'", event.ConfigurationId, event.WebhookId)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}

	bc := BindingContext{
		Binding:         link.BindingName,
		AdmissionReview: event.Review,
	}
	bc.Metadata.BindingType = KubernetesValidating
	bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
	bc.Metadata.Group = link.Group

	return BindingExecutionInfo{
		BindingContext:   []BindingContext{bc},
		Binding:          link.BindingName,
		IncludeSnapshots: link.IncludeSnapshots,
		Group:            link.Group,
	}
}
