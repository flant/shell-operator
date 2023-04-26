package admission

import (
	v1 "k8s.io/api/admissionregistration/v1"

	"github.com/flant/shell-operator/pkg/utils/string_helper"
)

// ConfigurationId is a first element in Path field for each Webhook.
// It should be url safe.
//
// WebhookId is a second element for Path field.

type Metadata struct {
	Name            string
	WebhookId       string
	ConfigurationId string // A suffix to create different ValidatingWebhookConfiguration/MutatingWebhookConfiguration resources.
	DebugName       string
	LogLabels       map[string]string
	MetricLabels    map[string]string
}

type IWebhookConfig interface {
	GetMeta() Metadata
	SetMeta(Metadata)
	SetClientConfig(v1.WebhookClientConfig)
	UpdateIds(string, string)
}

type ValidatingWebhookConfig struct {
	*v1.ValidatingWebhook
	Metadata
}

func (c *ValidatingWebhookConfig) GetMeta() Metadata  { return c.Metadata }
func (c *ValidatingWebhookConfig) SetMeta(m Metadata) { c.Metadata = m }
func (c *ValidatingWebhookConfig) SetClientConfig(cc v1.WebhookClientConfig) {
	equivalent := v1.Equivalent
	c.MatchPolicy = &equivalent
	c.ClientConfig = cc
}

type MutatingWebhookConfig struct {
	*v1.MutatingWebhook
	Metadata
}

func (c *MutatingWebhookConfig) GetMeta() Metadata  { return c.Metadata }
func (c *MutatingWebhookConfig) SetMeta(m Metadata) { c.Metadata = m }
func (c *MutatingWebhookConfig) SetClientConfig(cc v1.WebhookClientConfig) {
	equivalent := v1.Equivalent
	c.MatchPolicy = &equivalent
	c.ClientConfig = cc
}

// UpdateIds use confId and webhookId to set a ConfigurationId prefix and a WebhookId.
func (c *ValidatingWebhookConfig) UpdateIds(confID, webhookID string) {
	c.Metadata.ConfigurationId = confID
	if confID == "" {
		c.Metadata.ConfigurationId = DefaultConfigurationId
	}
	c.Metadata.WebhookId = string_helper.SafeURLString(webhookID)
}

func (c *MutatingWebhookConfig) UpdateIds(confID, webhookID string) {
	c.Metadata.ConfigurationId = confID
	if confID == "" {
		c.Metadata.ConfigurationId = DefaultConfigurationId
	}
	c.Metadata.WebhookId = string_helper.SafeURLString(webhookID)
}
