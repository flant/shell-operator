package kubeeventsmanager

import (
	"github.com/flant/shell-operator/pkg/kube/dedupclient"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type legacyTestClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
}

func testDedupClientFromLegacy(client legacyTestClient) *dedupclient.Client {
	return dedupclient.NewFromClients(client, client.Dynamic(), nil, memory.NewMemCacheClient(client.Discovery()))
}
