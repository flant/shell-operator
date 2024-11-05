package controller

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/admission/v1"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/webhook/admission"
)

// AdmissionBindingToWebhookLink is a link between a hook and a webhook configuration.
type AdmissionBindingToWebhookLink struct {
	BindingType     BindingType
	BindingName     string
	ConfigurationId string
	WebhookId       string
	// Useful fields to create a BindingContext
	IncludeSnapshots []string
	Group            string
}

type AdmissionBindingsController struct {
	// Controller holds validating/mutating bindings from one hook. Hook always belongs to one configurationId.
	ConfigurationId string
	// WebhookId -> link
	AdmissionLinks map[string]*AdmissionBindingToWebhookLink

	ValidatingBindings []ValidatingConfig
	MutatingBindings   []MutatingConfig

	webhookManager *admission.WebhookManager
}

// NewValidatingBindingsController returns an implementation of AdmissionBindingsController
var NewValidatingBindingsController = func() *AdmissionBindingsController {
	return &AdmissionBindingsController{
		AdmissionLinks: make(map[string]*AdmissionBindingToWebhookLink),
	}
}

func (c *AdmissionBindingsController) WithValidatingBindings(bindings []ValidatingConfig) {
	c.ValidatingBindings = bindings
}

func (c *AdmissionBindingsController) WithMutatingBindings(bindings []MutatingConfig) {
	c.MutatingBindings = bindings
}

func (c *AdmissionBindingsController) WithWebhookManager(mgr *admission.WebhookManager) {
	c.webhookManager = mgr
}

func (c *AdmissionBindingsController) EnableValidatingBindings() {
	confId := ""

	if len(c.ValidatingBindings) == 0 {
		return
	}

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
		c.AdmissionLinks[config.Webhook.Metadata.WebhookId] = &AdmissionBindingToWebhookLink{
			BindingType:      KubernetesValidating,
			BindingName:      config.BindingName,
			ConfigurationId:  c.ConfigurationId,
			WebhookId:        config.Webhook.Metadata.WebhookId,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			Group:            config.Group,
		}
		c.webhookManager.AddValidatingWebhook(config.Webhook)
	}
}

func (c *AdmissionBindingsController) EnableMutatingBindings() {
	confId := ""

	if len(c.MutatingBindings) == 0 {
		return
	}

	for _, config := range c.MutatingBindings {
		if config.Webhook.Metadata.ConfigurationId == "" && confId == "" {
			continue
		}
		if config.Webhook.Metadata.ConfigurationId != "" && confId == "" {
			confId = config.Webhook.Metadata.ConfigurationId
			continue
		}
		if config.Webhook.Metadata.ConfigurationId != confId {
			log.Errorf("Possible bug!!! kubernetesMutating has non-unique configurationIds: '%s' '%s'", config.Webhook.Metadata.ConfigurationId, confId)
		}
	}
	c.ConfigurationId = confId

	for _, config := range c.MutatingBindings {
		c.AdmissionLinks[config.Webhook.Metadata.WebhookId] = &AdmissionBindingToWebhookLink{
			BindingType:      KubernetesMutating,
			BindingName:      config.BindingName,
			ConfigurationId:  c.ConfigurationId,
			WebhookId:        config.Webhook.Metadata.WebhookId,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			Group:            config.Group,
		}
		c.webhookManager.AddMutatingWebhook(config.Webhook)
	}
}

func (c *AdmissionBindingsController) DisableValidatingBindings() {
	// TODO dynamic enable/disable validating webhooks.
}

func (c *AdmissionBindingsController) DisableMutatingBindings() {
	// TODO dynamic enable/disable mutating webhooks.
}

func (c *AdmissionBindingsController) CanHandleEvent(event admission.Event) bool {
	if c.ConfigurationId != event.ConfigurationId {
		return false
	}
	_, has := c.AdmissionLinks[event.WebhookId]
	return has
}

func (c *AdmissionBindingsController) HandleEvent(event admission.Event) BindingExecutionInfo {
	if c.ConfigurationId != event.ConfigurationId {
		log.Errorf("Possible bug!!! Unknown validating event: no binding for configurationId '%s' (webhookId '%s')", event.ConfigurationId, event.WebhookId)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}

	link, hasKey := c.AdmissionLinks[event.WebhookId]
	if !hasKey {
		log.Errorf("Possible bug!!! Unknown validating event: no binding for configurationId '%s', webhookId '%s'", event.ConfigurationId, event.WebhookId)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}

	bc := BindingContext{
		Binding:         link.BindingName,
		AdmissionReview: &v1.AdmissionReview{Request: event.Request},
	}
	bc.Metadata.BindingType = link.BindingType
	bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
	bc.Metadata.Group = link.Group

	return BindingExecutionInfo{
		BindingContext:   []BindingContext{bc},
		Binding:          link.BindingName,
		IncludeSnapshots: link.IncludeSnapshots,
		Group:            link.Group,
	}
}
