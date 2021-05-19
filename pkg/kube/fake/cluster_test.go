package fake

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRegisterCRD(t *testing.T) {
	f := NewFakeCluster("")

	f.RegisterCRD("deckhouse.io", "v1alpha1", "KeepalivedInstance", false)
	gvk := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: Pluralize("KeepalivedInstance"),
	}
	_, err := f.KubeClient.Dynamic().Resource(gvk).Namespace("").List(context.TODO(), v1.ListOptions{})
	require.NoError(t, err)
}
