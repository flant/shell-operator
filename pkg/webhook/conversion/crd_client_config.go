package conversion

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
)

// A clientConfig for a particular CRD.
type CrdClientConfig struct {
	KubeClient  kube.KubernetesClient
	CrdName     string
	Namespace   string
	ServiceName string
	Path        string
	CABundle    []byte
}

var SupportedConversionReviewVersions = []string{"v1", "v1beta1"}

func (c *CrdClientConfig) Update() error {
	client := c.KubeClient

	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + c.CrdName,
	}

	crdList, err := client.ApiExt().CustomResourceDefinitions().List(listOpts)
	if err != nil {
		return err
	}

	if len(crdList.Items) == 0 {
		return fmt.Errorf("crd/%s not found", c.CrdName)
	}

	crd := crdList.Items[0]

	if crd.Spec.Conversion == nil {
		crd.Spec.Conversion = new(extv1.CustomResourceConversion)
	}
	conv := crd.Spec.Conversion

	conv.Strategy = extv1.WebhookConverter
	if conv.Webhook == nil {
		conv.Webhook = new(extv1.WebhookConversion)
	}

	webhook := conv.Webhook

	webhook.ClientConfig = &extv1.WebhookClientConfig{
		URL: nil,
		Service: &extv1.ServiceReference{
			Namespace: c.Namespace,
			Name:      c.ServiceName,
			Path:      &c.Path,
		},
		CABundle: c.CABundle,
	}

	webhook.ConversionReviewVersions = SupportedConversionReviewVersions

	_, err = client.ApiExt().CustomResourceDefinitions().Update(&crd)
	if err != nil {
		return err
	}

	log.Infof("crd/%s spec.conversion is updated to a webhook behind %s/%s", c.CrdName, c.ServiceName, c.Path)

	return nil
}
