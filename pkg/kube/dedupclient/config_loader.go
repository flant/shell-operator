package dedupclient

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubeTokenFilePath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	kubeNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func buildRESTConfig(cfg Config) (*rest.Config, string, error) {
	var (
		restCfg   *rest.Config
		defaultNS = "default"
		err       error
	)

	switch {
	case cfg.RESTConfig != nil:
		if cfg.RESTConfig.Host == "" {
			return nil, "", fmt.Errorf("rest config host can't be empty")
		}
		restCfg = rest.CopyConfig(cfg.RESTConfig)
	case cfg.Server != "":
		restCfg = &rest.Config{Host: cfg.Server}
		_ = rest.SetKubernetesDefaults(restCfg)
	default:
		var outOfClusterErr error
		restCfg, defaultNS, outOfClusterErr = getOutOfClusterConfig(cfg.Context, cfg.Config)
		if restCfg == nil {
			if hasInClusterConfig() {
				restCfg, defaultNS, err = getInClusterConfig()
				if err != nil {
					if cfg.Config != "" || cfg.Context != "" {
						if outOfClusterErr != nil {
							return nil, "", fmt.Errorf("out-of-cluster config error: %v, in-cluster config error: %v", outOfClusterErr, err)
						}
						return nil, "", fmt.Errorf("in-cluster config is not found")
					}
					return nil, "", err
				}
			} else if outOfClusterErr != nil {
				return nil, "", outOfClusterErr
			} else {
				return nil, "", fmt.Errorf("no kubernetes client config found")
			}
		}
	}

	if restCfg == nil {
		return nil, "", fmt.Errorf("failed to initialize kubernetes client: no valid configuration found")
	}
	if cfg.QPS != 0 {
		restCfg.QPS = cfg.QPS
	}
	if cfg.Burst != 0 {
		restCfg.Burst = cfg.Burst
	}
	if cfg.Timeout != 0 {
		restCfg.Timeout = cfg.Timeout
	}
	return restCfg, defaultNS, nil
}

func newClientConfig(contextName, kubeconfig string) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}

func getOutOfClusterConfig(contextName, configPath string) (*rest.Config, string, error) {
	clientConfig := newClientConfig(contextName, configPath)

	defaultNS, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("cannot determine default kubernetes namespace: %s", err)
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", makeOutOfClusterClientConfigError(configPath, contextName, err)
	}
	return config, defaultNS, nil
}

func makeOutOfClusterClientConfigError(kubeConfig, kubeContext string, err error) error {
	baseErrMsg := "out-of-cluster configuration problem"
	if kubeConfig != "" {
		baseErrMsg += fmt.Sprintf(", custom kube config path is '%s'", kubeConfig)
	}
	if kubeContext != "" {
		baseErrMsg += fmt.Sprintf(", custom kube context is '%s'", kubeContext)
	}
	return fmt.Errorf("%s: %s", baseErrMsg, err)
}

func hasInClusterConfig() bool {
	token, _ := fileExists(kubeTokenFilePath)
	ns, _ := fileExists(kubeNamespaceFilePath)
	return token && ns
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getInClusterConfig() (*rest.Config, string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, "", fmt.Errorf("in-cluster configuration problem: %s", err)
	}

	data, err := os.ReadFile(kubeNamespaceFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("in-cluster configuration problem: cannot determine default kubernetes namespace: error reading %s: %s", kubeNamespaceFilePath, err)
	}
	return config, string(data), nil
}

func newCachedDiscovery(config *rest.Config) (discovery.CachedDiscoveryInterface, error) {
	if _, inMemory := os.LookupEnv("FLANT_KUBE_CLIENT_IN_MEMORY_DISCOVERY_CACHE"); inMemory {
		d, err := discovery.NewDiscoveryClientForConfig(config)
		if err != nil {
			return nil, err
		}
		return memory.NewMemCacheClient(d), nil
	}

	cacheDiscoveryDir, err := os.MkdirTemp("", "kube-cache-discovery-*")
	if err != nil {
		return nil, err
	}
	cacheHTTPDir, err := os.MkdirTemp("", "kube-cache-http-*")
	if err != nil {
		return nil, err
	}
	return disk.NewCachedDiscoveryClientForConfig(config, cacheDiscoveryDir, cacheHTTPDir, 10*time.Minute)
}

func newRESTMapper(cachedDiscovery discovery.CachedDiscoveryInterface, logger *log.Logger) meta.RESTMapper {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)
	return restmapper.NewShortcutExpander(mapper, cachedDiscovery, func(warning string) {
		if logger != nil {
			logger.Warn("warning", slog.String("warning", warning))
		}
	})
}

func (c *Client) discovery() discovery.DiscoveryInterface {
	if c.cachedDiscovery != nil {
		return c.cachedDiscovery
	}
	return c.Discovery()
}

func (c *Client) apiResourceList(apiVersion string) ([]*metav1.APIResourceList, error) {
	if apiVersion == "" {
		switch c.discovery().(type) {
		case *fakediscovery.FakeDiscovery:
			_, res, err := c.discovery().ServerGroupsAndResources()
			return res, err
		default:
			return c.discovery().ServerPreferredResources()
		}
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("apiVersion %q is invalid", apiVersion)
	}

	list, err := c.discovery().ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return nil, errors.Wrapf(err, "apiVersion %q has no supported resources in cluster", apiVersion)
	}
	return []*metav1.APIResourceList{list}, nil
}

func (c *Client) apiResource(apiVersion, kind string) (*metav1.APIResource, error) {
	lists, err := c.APIResourceList(apiVersion)
	if err != nil && len(lists) == 0 {
		return nil, err
	}

	resource := getAPIResourceFromResourceLists(kind, lists)
	if resource == nil {
		gv, _ := schema.ParseGroupVersion(apiVersion)
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: gv.Group, Resource: kind}, "")
	}
	return resource, nil
}

func getAPIResourceFromResourceLists(kind string, resourceLists []*metav1.APIResourceList) *metav1.APIResource {
	for _, list := range resourceLists {
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			if equalLowerCasedToOneOf(kind, append(resource.ShortNames, resource.Kind, resource.Name)...) {
				gv, _ := schema.ParseGroupVersion(list.GroupVersion)
				resource.Group = gv.Group
				resource.Version = gv.Version
				return &resource
			}
		}
	}
	return nil
}

func equalLowerCasedToOneOf(term string, choices ...string) bool {
	if len(choices) == 0 {
		return false
	}
	lTerm := strings.ToLower(term)
	for _, choice := range choices {
		if lTerm == strings.ToLower(choice) {
			return true
		}
	}
	return false
}
