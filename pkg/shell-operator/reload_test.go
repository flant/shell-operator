package shell_operator

import (
	"context"
	"fmt"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/webhook/admission"
)

func newReloadTestOperator(t *testing.T, hm *stubHookManager) (*ShellOperator, *klient.Client) {
	t.Helper()

	kubeClient := klient.NewFake(nil)
	op := NewBareShellOperator(context.Background(), WithLogger(log.NewNop()))
	op.HookManager = hm
	op.KubeClient = kubeClient
	op.AdmissionWebhookManager = admission.NewWebhookManager(kubeClient, admission.WithLogger(log.NewNop()))

	return op, kubeClient
}

func addValidatingResource(t *testing.T, op *ShellOperator, kubeClient *klient.Client, confID, name string) {
	t.Helper()

	_, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Create(context.Background(), &admissionregv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	op.AdmissionWebhookManager.ValidatingResources[confID] = admission.NewValidatingWebhookResource(
		admission.WebhookResourceOptions{
			KubeClient:        kubeClient,
			ConfigurationName: name,
		},
		log.NewNop(),
	)
}

// A failing HookManager.Init must leave the current registrations intact:
// clearing them first would strand the live webhook configurations with no
// way to unregister them.
func TestReloadHooksKeepsRegistrationsWhenInitFails(t *testing.T) {
	op, kubeClient := newReloadTestOperator(t, &stubHookManager{
		initFunc: func() error { return fmt.Errorf("malformed hook") },
	})
	addValidatingResource(t, op, kubeClient, "hooks", "test-hooks")

	err := op.ReloadHooks(context.Background())

	require.ErrorContains(t, err, "re-init hook manager")
	assert.Len(t, op.AdmissionWebhookManager.ValidatingResources, 1)
}

// After a successful reload that discovers no validating hooks, the previously
// registered ValidatingWebhookConfiguration must be deleted from the cluster.
func TestReloadHooksUnregistersStaleValidatingWebhook(t *testing.T) {
	op, kubeClient := newReloadTestOperator(t, &stubHookManager{})
	addValidatingResource(t, op, kubeClient, "hooks", "test-hooks")

	require.NoError(t, op.ReloadHooks(context.Background()))

	assert.Empty(t, op.AdmissionWebhookManager.ValidatingResources)

	_, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Get(context.Background(), "test-hooks", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "stale configuration must be deleted, got %v", err)
}

func TestReloadHooksWithoutHookManager(t *testing.T) {
	op := NewBareShellOperator(context.Background(), WithLogger(log.NewNop()))

	require.ErrorContains(t, op.ReloadHooks(context.Background()), "hook manager is not initialized")
}
