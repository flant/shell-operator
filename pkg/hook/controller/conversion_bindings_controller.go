package controller

import (
	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// A link between a hook and a kube monitor
type ConversionBindingToWebhookLink struct {
	BindingName string
	// Useful fields to create a BindingContext
	CrdName          string
	FromVersion      string
	ToVersion        string
	IncludeSnapshots []string
	Group            string
}

// ScheduleBindingsController handles schedule bindings for one hook.
type ConversionBindingsController interface {
	WithBindings([]ConversionConfig)
	WithWebhookManager(manager *conversion.WebhookManager)
	EnableConversionBindings()
	DisableConversionBindings()
	CanHandleEvent(event conversion.Event, rule conversion.Rule) bool
	HandleEvent(event conversion.Event, rule conversion.Rule) BindingExecutionInfo
}

// Controller holds validating bindings from one hook.
type conversionBindingsController struct {
	// crdName -> conversionRule id -> link
	Links map[string]map[conversion.Rule]*ConversionBindingToWebhookLink

	Bindings []ConversionConfig

	webhookManager *conversion.WebhookManager
}

var _ ConversionBindingsController = &conversionBindingsController{}

// NewConversionBindingsController returns an implementation of ConversionBindingsController
var NewConversionBindingsController = func() *conversionBindingsController {
	return &conversionBindingsController{
		Links: make(map[string]map[conversion.Rule]*ConversionBindingToWebhookLink),
	}
}

func (c *conversionBindingsController) WithBindings(bindings []ConversionConfig) {
	c.Bindings = bindings
}

func (c *conversionBindingsController) WithWebhookManager(mgr *conversion.WebhookManager) {
	c.webhookManager = mgr
}

func (c *conversionBindingsController) EnableConversionBindings() {
	// Setup links and inform webhookManager about webhooks.
	for _, config := range c.Bindings {
		if _, ok := c.Links[config.Webhook.CrdName]; !ok {
			c.Links[config.Webhook.CrdName] = map[conversion.Rule]*ConversionBindingToWebhookLink{}
		}
		for _, conv := range config.Webhook.Rules {
			c.Links[config.Webhook.CrdName][conv] = &ConversionBindingToWebhookLink{
				BindingName:      config.BindingName,
				IncludeSnapshots: config.IncludeSnapshotsFrom,
				Group:            config.Group,
				FromVersion:      conv.FromVersion,
				ToVersion:        conv.ToVersion,
			}
		}
		log.Infof("conversion binding controller: add webhook from config: %v", config)
		c.webhookManager.AddWebhook(config.Webhook)
	}
}

func (c *conversionBindingsController) DisableConversionBindings() {
	// TODO dynamic enable/disable conversion webhooks.
}

func (c *conversionBindingsController) CanHandleEvent(event conversion.Event, rule conversion.Rule) bool {
	_, has := c.Links[event.CrdName]
	if !has {
		return false
	}
	_, has = c.Links[event.CrdName][rule]
	return has
}

func (c *conversionBindingsController) HandleEvent(event conversion.Event, rule conversion.Rule) BindingExecutionInfo {
	_, hasKey := c.Links[event.CrdName]
	if !hasKey {
		log.Errorf("Possible bug!!! No binding for conversion event for crd/%s", event.CrdName)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}
	link, has := c.Links[event.CrdName][rule]
	if !has {
		log.Errorf("Possible bug!!! Event has an unknown conversion rule %s for crd/%s: no binding was registered", rule.String(), event.CrdName)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
			AllowFailure:   false,
		}
	}

	bc := BindingContext{
		Binding:          link.BindingName,
		ConversionReview: event.GetReview(),
		FromVersion:      link.FromVersion,
		ToVersion:        link.ToVersion,
	}
	bc.Metadata.BindingType = KubernetesConversion
	bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
	bc.Metadata.Group = link.Group

	return BindingExecutionInfo{
		BindingContext:   []BindingContext{bc},
		Binding:          link.BindingName,
		IncludeSnapshots: link.IncludeSnapshots,
		Group:            link.Group,
	}
}
