package validating_webhook

import (
	"regexp"
	"strings"

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
func (c *ValidatingWebhookConfig) UpdateIds(confId, webhookId string) {
	c.Metadata.ConfigurationId = confId
	if confId == "" {
		c.Metadata.ConfigurationId = DefaultConfigurationId
	}
	c.Metadata.WebhookId = safeUrlString(webhookId)
}

var safeReList = []*regexp.Regexp{
	regexp.MustCompile(`([A-Z])`),
	regexp.MustCompile(`[^a-z0-9-/]`),
	regexp.MustCompile(`[-]+`),
}

func safeUrlString(s string) string {
	s = safeReList[0].ReplaceAllString(s, "-$1")
	s = strings.ToLower(s)
	s = safeReList[1].ReplaceAllString(s, "-")
	return safeReList[2].ReplaceAllString(s, "-")
}
