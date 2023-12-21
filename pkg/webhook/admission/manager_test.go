package admission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1"
)

func Test_Manager_AddWebhook(t *testing.T) {
	m := NewWebhookManager(nil)
	m.Namespace = "default"
	vs := &WebhookSettings{}
	vs.ConfigurationName = "webhook-configuration"
	vs.ServiceName = "webhook-service"
	vs.ServerKeyPath = "testdata/demo-certs/server-key.pem"
	vs.ServerCertPath = "testdata/demo-certs/server.crt"
	vs.CAPath = "testdata/demo-certs/ca.pem"
	m.Settings = vs
	m.DefaultFailurePolicy = v1.Ignore

	err := m.Init()
	if err != nil {
		t.Fatalf("WebhookManager should init: %v", err)
	}

	t.Run("Webhook with set FailurePolicy", func(t *testing.T) {
		fail := v1.Fail
		none := v1.SideEffectClassNone
		timeoutSeconds := int32(10)

		cfg := &ValidatingWebhookConfig{
			ValidatingWebhook: &v1.ValidatingWebhook{
				Name: "test-validating",
				Rules: []v1.RuleWithOperations{
					{
						Operations: []v1.OperationType{v1.OperationAll},
						Rule: v1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				},
				FailurePolicy:  &fail,
				SideEffects:    &none,
				TimeoutSeconds: &timeoutSeconds,
			},
		}
		m.AddValidatingWebhook(cfg)

		if len(m.ValidatingResources) != 1 {
			t.Fatalf("WebhookManager should have resources: got length %d", len(m.ValidatingResources))
		}

		for k, v := range m.ValidatingResources {
			if len(v.hooks) != 1 {
				t.Fatalf("Resource '%s' should have Webhooks: got length %d", k, len(m.ValidatingResources))
			}
			assert.Equal(t, v1.Fail, *v.hooks[""].FailurePolicy)
		}
	})

	t.Run("Webhook with default FailurePolicy", func(t *testing.T) {
		none := v1.SideEffectClassNone
		timeoutSeconds := int32(10)

		cfg := &ValidatingWebhookConfig{
			ValidatingWebhook: &v1.ValidatingWebhook{
				Name: "test-validating",
				Rules: []v1.RuleWithOperations{
					{
						Operations: []v1.OperationType{v1.OperationAll},
						Rule: v1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				},
				SideEffects:    &none,
				TimeoutSeconds: &timeoutSeconds,
			},
		}
		m.AddValidatingWebhook(cfg)

		if len(m.ValidatingResources) != 1 {
			t.Fatalf("WebhookManager should have resources: got length %d", len(m.ValidatingResources))
		}

		for k, v := range m.ValidatingResources {
			if len(v.hooks) != 1 {
				t.Fatalf("Resource '%s' should have Webhooks: got length %d", k, len(m.ValidatingResources))
			}
			assert.Equal(t, v1.Ignore, *v.hooks[""].FailurePolicy)
		}
	})
}
