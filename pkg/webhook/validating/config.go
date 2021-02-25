package validating

import (
	"github.com/flant/shell-operator/pkg/utils/string_helper"
	v1 "k8s.io/api/admissionregistration/v1"
)

// ConfigurationId is a first element in Path field for each Webhook.
// It should be url safe.
//
// WebhookId is a second element for Path field.

// ValidatingWebhookConfig
type ValidatingWebhookConfig struct {
	*v1.ValidatingWebhook
	Metadata struct {
		Name            string
		WebhookId       string
		ConfigurationId string // A suffix to create different ValidatingWebhookConfiguration resources.
		DebugName       string
		LogLabels       map[string]string
		MetricLabels    map[string]string
	}
}

// UpdateIds use confId and webhookId to set a ConfigurationId prefix and a WebhookId.
func (c *ValidatingWebhookConfig) UpdateIds(confID, webhookID string) {
	c.Metadata.ConfigurationId = confID
	if confID == "" {
		c.Metadata.ConfigurationId = DefaultConfigurationId
	}
	c.Metadata.WebhookId = string_helper.SafeURLString(webhookID)
}
