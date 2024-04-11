package conversion

import (
	"context"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	klient "github.com/flant/kube-client/client"
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

func (c *CrdClientConfig) Update(ctx context.Context) error {
	var (
		retryTimeout = 15 * time.Second
		retryBudget  = 60 // 60 times * 15 sec = 15 min
		client       = c.KubeClient
	)

tryToGetCRD:
	crd, err := client.ApiExt().CustomResourceDefinitions().Get(ctx, c.CrdName, metav1.GetOptions{})
	if err != nil {
		if retryBudget > 0 {
			retryBudget--
			time.Sleep(retryTimeout)
			goto tryToGetCRD
		}

		return err
	}

	if crd.Spec.Conversion == nil {
		crd.Spec.Conversion = new(extv1.CustomResourceConversion)
	}
	crd.Spec.Conversion.Strategy = extv1.WebhookConverter

	if crd.Spec.Conversion.Webhook == nil {
		crd.Spec.Conversion.Webhook = new(extv1.WebhookConversion)
	}
	crd.Spec.Conversion.Webhook.ClientConfig = &extv1.WebhookClientConfig{
		URL: nil,
		Service: &extv1.ServiceReference{
			Namespace: c.Namespace,
			Name:      c.ServiceName,
			Path:      &c.Path,
		},
		CABundle: c.CABundle,
	}
	crd.Spec.Conversion.Webhook.ConversionReviewVersions = SupportedConversionReviewVersions

	_, err = client.ApiExt().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
