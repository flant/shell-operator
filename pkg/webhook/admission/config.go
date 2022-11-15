package admission

import (
	"github.com/flant/shell-operator/pkg/utils/string_helper"
	v1 "k8s.io/api/admissionregistration/v1"
)

// ConfigurationId is a first element in Path field for each Webhook.
// It should be url safe.
//
// WebhookId is a second element for Path field.

type Metadata struct {
	Name            string
	WebhookId       string
	ConfigurationId string // A suffix to create different ValidatingWebhookConfiguration resources.
	DebugName       string
	LogLabels       map[string]string
	MetricLabels    map[string]string
}

type IWebhookConfig interface {
	GetMeta() Metadata
	SetMeta(Metadata)
	SetClientConfig(v1.WebhookClientConfig)
}

// ValidatingWebhookConfig
type ValidatingWebhookConfig struct {
	*v1.ValidatingWebhook
	Metadata
}

func (wc *ValidatingWebhookConfig) GetMeta() Metadata  { return wc.Metadata }
func (wc *ValidatingWebhookConfig) SetMeta(m Metadata) { wc.Metadata = wc.Metadata }
func (wc *ValidatingWebhookConfig) SetClientConfig(cc v1.WebhookClientConfig) {
	equivalent := v1.Equivalent
	wc.MatchPolicy = &equivalent
	wc.ClientConfig = cc
}

// ValidatingWebhookConfig
type MutatingWebhookConfig struct {
	*v1.MutatingWebhook
	Metadata
}

func (wc *MutatingWebhookConfig) GetMeta() Metadata  { return wc.Metadata }
func (wc *MutatingWebhookConfig) SetMeta(m Metadata) { wc.Metadata = wc.Metadata }
func (wc *MutatingWebhookConfig) SetClientConfig(cc v1.WebhookClientConfig) {
	equivalent := v1.Equivalent
	wc.MatchPolicy = &equivalent
	wc.ClientConfig = cc
}

// UpdateIds use confId and webhookId to set a ConfigurationId prefix and a WebhookId.
func (c *ValidatingWebhookConfig) UpdateIds(confID, webhookID string) {
	c.Metadata.ConfigurationId = confID
	if confID == "" {
		c.Metadata.ConfigurationId = DefaultConfigurationId
	}
	c.Metadata.WebhookId = string_helper.SafeURLString(webhookID)
}
