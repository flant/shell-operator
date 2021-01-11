package validating_webhook

import (
	"testing"

	v1 "k8s.io/api/admissionregistration/v1"

	"github.com/flant/shell-operator/pkg/app"
)

func Test_Manager_AddWebhook(t *testing.T) {
	m := NewWebhookManager()
	app.Namespace = "default"
	vs := app.ValidatingWebhookSettings
	vs.ConfigurationName = "webhook-configuration"
	vs.ServiceName = "webhook-service"
	vs.ServerKeyPath = "testdata/demo-certs/server-key.pem"
	vs.ServerCertPath = "testdata/demo-certs/server.crt"
	vs.CAPath = "testdata/demo-certs/ca.pem"

	err := m.Init()

	if err != nil {
		t.Fatalf("WebhookManager should init: %v", err)
	}

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
	m.AddWebhook(cfg)

	if len(m.Resources) != 1 {
		t.Fatalf("WebhookManager should have resources: got length %d", len(m.Resources))
	}

	for k, v := range m.Resources {
		if len(v.Webhooks) != 1 {
			t.Fatalf("Resource '%s' should have Webhooks: got length %d", k, len(m.Resources))
		}
	}
}
