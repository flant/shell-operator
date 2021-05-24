package fake

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestPluralize(t *testing.T) {
	tests := map[string]string{
		"Endpoints":             "endpoints",
		"Prometheus":            "prometheuses",
		"NetworkPolicy":         "networkpolicies",
		"CustomPrometheusRules": "customprometheusrules",
		"Alertmanager":          "alertmanagers",
		"Node":                  "nodes",
	}

	for before, after := range tests {
		assert.Equal(t, after, Pluralize(before))
	}
}
