package dedupclient

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/ldmonster/kubeclient/store"
	apixfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/metadata"
	fakemetadata "k8s.io/client-go/metadata/fake"
)

// NewFake constructs a singleton client backed by client-go fakes. It is meant
// for unit tests that exercise shell-operator code paths using typed/dynamic
// Kubernetes APIs without starting the upstream dedup cache.
func NewFake(gvrToListKind map[schema.GroupVersionResource]string) *Client {
	scheme := runtime.NewScheme()
	kube := fake.NewSimpleClientset()
	dyn := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	metaClient := fakemetadata.NewSimpleMetadataClient(scheme)
	apiExt := apixfake.NewSimpleClientset()

	return &Client{
		Interface:       kube,
		logger:          log.NewNop(),
		defaultNS:       "default",
		dynamicClient:   dyn,
		apiExtClient:    apiExt.ApiextensionsV1(),
		metadataClient:  metadata.Interface(metaClient),
		cachedDiscovery: memory.NewMemCacheClient(kube.Discovery()),
		informerStores:  make(map[string]store.Store),
		done:            make(chan struct{}),
	}
}

func NewFromClients(kube kubernetes.Interface, dyn dynamic.Interface, apiExtClient apixv1client.ApiextensionsV1Interface, cachedDiscovery discovery.CachedDiscoveryInterface) *Client {
	if cachedDiscovery == nil && kube != nil {
		cachedDiscovery = memory.NewMemCacheClient(kube.Discovery())
	}
	if apiExtClient == nil {
		apiExtClient = apixfake.NewSimpleClientset().ApiextensionsV1()
	}
	return &Client{
		Interface:       kube,
		logger:          log.NewNop(),
		defaultNS:       "default",
		dynamicClient:   dyn,
		apiExtClient:    apiExtClient,
		cachedDiscovery: cachedDiscovery,
		informerStores:  make(map[string]store.Store),
		done:            make(chan struct{}),
	}
}

// ReloadDynamic replaces the fake dynamic client with a new resource/list-kind
// mapping. It mirrors the old flant kube-client fake helper used by tests.
func (c *Client) ReloadDynamic(gvrToListKind map[schema.GroupVersionResource]string) {
	scheme := runtime.NewScheme()
	c.dynamicClient = fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
}

func (c *Client) ReplaceDynamic(dynamicClient dynamic.Interface) {
	c.dynamicClient = dynamicClient
}
