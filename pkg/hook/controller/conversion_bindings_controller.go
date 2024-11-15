package controller

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
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

// ConversionBindingsController holds validating bindings from one hook.
type ConversionBindingsController struct {
	// crdName -> conversionRule id -> link
	Links map[string]map[conversion.Rule]*ConversionBindingToWebhookLink

	Bindings []htypes.ConversionConfig

	webhookManager *conversion.WebhookManager
}

// NewConversionBindingsController returns an implementation of ConversionBindingsController
var NewConversionBindingsController = func() *ConversionBindingsController {
	return &ConversionBindingsController{
		Links: make(map[string]map[conversion.Rule]*ConversionBindingToWebhookLink),
	}
}

func (c *ConversionBindingsController) WithBindings(bindings []htypes.ConversionConfig) {
	c.Bindings = bindings
}

func (c *ConversionBindingsController) WithWebhookManager(mgr *conversion.WebhookManager) {
	c.webhookManager = mgr
}

func (c *ConversionBindingsController) EnableConversionBindings() {
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

func (c *ConversionBindingsController) DisableConversionBindings() {
	// TODO dynamic enable/disable conversion webhooks.
}

func (c *ConversionBindingsController) CanHandleEvent(crdName string, _ *v1.ConversionRequest, rule conversion.Rule) bool {
	_, has := c.Links[crdName]
	if !has {
		return false
	}
	_, has = c.Links[crdName][rule]
	return has
}

func (c *ConversionBindingsController) HandleEvent(crdName string, request *v1.ConversionRequest, rule conversion.Rule) BindingExecutionInfo {
	_, hasKey := c.Links[crdName]
	if !hasKey {
		log.Errorf("Possible bug!!! No binding for conversion event for crd/%s", crdName)
		return BindingExecutionInfo{
			BindingContext: []bctx.BindingContext{},
			AllowFailure:   false,
		}
	}
	link, has := c.Links[crdName][rule]
	if !has {
		log.Errorf("Possible bug!!! Event has an unknown conversion rule %s for crd/%s: no binding was registered", rule.String(), crdName)
		return BindingExecutionInfo{
			BindingContext: []bctx.BindingContext{},
			AllowFailure:   false,
		}
	}

	bc := bctx.BindingContext{
		Binding:          link.BindingName,
		ConversionReview: &v1.ConversionReview{Request: request},
		FromVersion:      link.FromVersion,
		ToVersion:        link.ToVersion,
	}
	bc.Metadata.BindingType = htypes.KubernetesConversion
	bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
	bc.Metadata.Group = link.Group

	return BindingExecutionInfo{
		BindingContext:   []bctx.BindingContext{bc},
		Binding:          link.BindingName,
		IncludeSnapshots: link.IncludeSnapshots,
		Group:            link.Group,
	}
}
