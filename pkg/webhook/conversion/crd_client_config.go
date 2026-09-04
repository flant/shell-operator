package conversion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg"
)

// A clientConfig for a particular CRD.
type CrdClientConfig struct {
	KubeClient  *klient.Client
	CrdName     string
	Namespace   string
	ServiceName string
	Path        string
	CABundle    []byte
}

var SupportedConversionReviewVersions = []string{"v1", "v1beta1"}

// PatchConversion points spec.conversion of the CRD at this operator's webhook
// server, leaving every other field of the CRD alone.
func (c *CrdClientConfig) PatchConversion(ctx context.Context) error {
	var (
		retryTimeout = 15 * time.Second
		retryBudget  = 60 // 60 times * 15 sec = 15 min
		client       = c.KubeClient
	)

	conv, err := json.Marshal(&extv1.CustomResourceConversion{
		Strategy: extv1.WebhookConverter,
		Webhook: &extv1.WebhookConversion{
			ClientConfig: &extv1.WebhookClientConfig{
				Service: &extv1.ServiceReference{
					Namespace: c.Namespace,
					Name:      c.ServiceName,
					Path:      &c.Path,
				},
				CABundle: c.CABundle,
			},
			ConversionReviewVersions: SupportedConversionReviewVersions,
		},
	})
	if err != nil {
		return fmt.Errorf("marshal conversion: %w", err)
	}

	patch := []byte(`[{"op":"add","path":"/spec/conversion","value":` + string(conv) + `}]`)

	// The CRD is often absent when a hook registers its conversion bindings, so the
	// patch is retried on the budget the Get used to hold.
	for {
		_, err = client.ApiExt().CustomResourceDefinitions().Patch(ctx, c.CrdName, types.JSONPatchType, patch, pkg.DefaultPatchOptions())
		if err == nil {
			return nil
		}

		if retryBudget == 0 {
			return fmt.Errorf("patch CRD conversion: %w", err)
		}
		retryBudget--

		select {
		case <-ctx.Done():
			return fmt.Errorf("patch CRD conversion: %w", ctx.Err())
		case <-time.After(retryTimeout):
		}
	}
}
