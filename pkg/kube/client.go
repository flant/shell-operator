package kube

// TODO do not copy, use import "github.com/flant/kubedog/pkg/kube"

import (
	"fmt"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (only required to authenticate against GKE clusters)
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// log klog messages from client-go with logrus
	_ "github.com/flant/shell-operator/pkg/utils/klogtologrus"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

const (
	kubeTokenFilePath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	kubeNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

type KubernetesClient interface {
	kubernetes.Interface

	WithContextName(contextName string)
	WithConfigPath(configPath string)
	WithRateLimiterSettings(qps float32, burst int)

	Init() error

	DefaultNamespace() string
	Dynamic() dynamic.Interface

	APIResourceList(apiVersion string) ([]*metav1.APIResourceList, error)
	APIResource(apiVersion string, kind string) (*metav1.APIResource, error)
	GroupVersionResource(apiVersion string, kind string) (schema.GroupVersionResource, error)
}

var NewKubernetesClient = func() KubernetesClient {
	return &kubernetesClient{}
}

func NewFakeKubernetesClient() KubernetesClient {
	scheme := runtime.NewScheme()
	objs := []runtime.Object{}

	return &kubernetesClient{
		Interface:        fake.NewSimpleClientset(),
		defaultNamespace: "default",
		dynamicClient:    fakedynamic.NewSimpleDynamicClient(scheme, objs...),
	}
}

var _ KubernetesClient = &kubernetesClient{}

type kubernetesClient struct {
	kubernetes.Interface
	contextName      string
	configPath       string
	defaultNamespace string
	dynamicClient    dynamic.Interface
	qps              float32
	burst            int
}

func (c *kubernetesClient) WithContextName(name string) {
	c.contextName = name
}

func (c *kubernetesClient) WithConfigPath(path string) {
	c.configPath = path
}

func (c *kubernetesClient) WithRateLimiterSettings(qps float32, burst int) {
	c.qps = qps
	c.burst = burst
}

func (c *kubernetesClient) DefaultNamespace() string {
	return c.defaultNamespace
}

func (c *kubernetesClient) Dynamic() dynamic.Interface {
	return c.dynamicClient
}

func (c *kubernetesClient) Init() error {
	logEntry := log.WithField("operator.component", "KubernetesAPIClient")

	var err error
	var config *rest.Config
	var configType = "out-of-cluster"
	var defaultNs string

	// Try to load from kubeconfig in flags or from ~/.kube/config
	config, defaultNs, outOfClusterErr := getOutOfClusterConfig(c.contextName, c.configPath)

	if config == nil {
		if hasInClusterConfig() {
			// Try to configure as inCluster
			config, defaultNs, err = getInClusterConfig()
			if err != nil {
				if c.configPath != "" || c.contextName != "" {
					if outOfClusterErr != nil {
						err = fmt.Errorf("out-of-cluster config error: %v, in-cluster config error: %v", outOfClusterErr, err)
						logEntry.Errorf("configuration problems: %s", err)
						return err
					} else {
						return fmt.Errorf("in-cluster config is not found")
					}
				} else {
					logEntry.Errorf("in-cluster problem: %s", err)
					return err
				}
			}
		} else {
			// if not in cluster return outOfCluster error
			if outOfClusterErr != nil {
				logEntry.Errorf("out-of-cluster problem: %s", outOfClusterErr)
				return outOfClusterErr
			} else {
				return fmt.Errorf("no kubernetes client config found")
			}
		}
		configType = "in-cluster"
	}

	c.defaultNamespace = defaultNs

	config.QPS = c.qps
	config.Burst = c.burst

	c.Interface, err = kubernetes.NewForConfig(config)
	if err != nil {
		logEntry.Errorf("configuration problem: %s", err)
		return err
	}

	c.dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	logEntry.Infof("Kubernetes client is configured successfully with '%s' config", configType)

	return nil
}

func makeOutOfClusterClientConfigError(kubeConfig, kubeContext string, err error) error {
	baseErrMsg := fmt.Sprintf("out-of-cluster configuration problem")

	if kubeConfig != "" {
		baseErrMsg += fmt.Sprintf(", custom kube config path is '%s'", kubeConfig)
	}

	if kubeContext != "" {
		baseErrMsg += fmt.Sprintf(", custom kube context is '%s'", kubeContext)
	}

	return fmt.Errorf("%s: %s", baseErrMsg, err)
}

func getClientConfig(context string, kubeconfig string) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	if context != "" {
		overrides.CurrentContext = context
	}

	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}

func hasInClusterConfig() bool {
	token, _ := utils_file.FileExists(kubeTokenFilePath)
	ns, _ := utils_file.FileExists(kubeNamespaceFilePath)
	return token && ns
}

func getOutOfClusterConfig(contextName string, configPath string) (config *rest.Config, defaultNs string, err error) {
	clientConfig := getClientConfig(contextName, configPath)

	defaultNs, _, err = clientConfig.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("cannot determine default kubernetes namespace: %s", err)
	}

	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, "", makeOutOfClusterClientConfigError(configPath, contextName, err)
	}

	//rc, err := clientConfig.RawConfig()
	//if err != nil {
	//	return nil, fmt.Errorf("cannot get raw kubernetes config: %s", err)
	//}
	//
	//if contextName != "" {
	//	Context = contextName
	//} else {
	//	Context = rc.CurrentContext
	//}

	return
}

func getInClusterConfig() (config *rest.Config, defaultNs string, err error) {
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, "", fmt.Errorf("in-cluster configuration problem: %s", err)
	}

	data, err := ioutil.ReadFile(kubeNamespaceFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("in-cluster configuration problem: cannot determine default kubernetes namespace: error reading %s: %s", kubeNamespaceFilePath, err)
	}
	defaultNs = string(data)

	return
}

// APIResourceList fetches lists of APIResource objects from cluster. It returns all preferred
// resources if apiVersion is empty. An array with one list is returned if apiVersion is valid.
//
// NOTE that fetching all preferred resources can give errors if there are non-working
// api controllers in cluster.
func (c *kubernetesClient) APIResourceList(apiVersion string) (lists []*metav1.APIResourceList, err error) {
	if apiVersion == "" {
		// Get all preferred resources.
		// Can return errors if api controllers are not available.
		return c.Discovery().ServerPreferredResources()
	} else {
		// Get only resources for desired group and version
		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			return nil, fmt.Errorf("apiVersion '%s' is invalid", apiVersion)
		}

		list, err := c.Discovery().ServerResourcesForGroupVersion(gv.String())
		if err != nil {
			return nil, fmt.Errorf("apiVersion '%s' has no supported resources in cluster: %v", apiVersion, err)
		}
		lists = []*metav1.APIResourceList{list}
	}

	// TODO should it copy group and version into each resource?

	// TODO create debug command to output this from cli
	// Debug mode will list all available CRDs for apiVersion
	//for _, r := range list.APIResources {
	//	log.Debugf("GVR: %30s %30s %30s", list.GroupVersion, r.Kind,
	//		fmt.Sprintf("%+v", append([]string{r.Name}, r.ShortNames...)),
	//	)
	//}

	return
}

// APIResource fetches APIResource object from cluster that specifies the name of a resource and whether it is namespaced.
//
// NOTE that fetching with empty apiVersion can give errors if there are non-working
// api controllers in cluster.
func (c *kubernetesClient) APIResource(apiVersion string, kind string) (res *metav1.APIResource, err error) {
	lists, err := c.APIResourceList(apiVersion)
	if err != nil && len(lists) == 0 {
		// apiVersion is defined and there is a ServerResourcesForGroupVersion error
		return nil, err
	}

	for _, list := range lists {
		for _, resource := range list.APIResources {
			// TODO is it ok to ignore resources with no verbs?
			if len(resource.Verbs) == 0 {
				continue
			}

			if equalLowerCasedToOneOf(kind, append(resource.ShortNames, resource.Kind, resource.Name)...) {
				gv, _ := schema.ParseGroupVersion(list.GroupVersion)
				resource.Group = gv.Group
				resource.Version = gv.Version
				return &resource, nil
			}
		}
	}

	// If resource is not found, append additional error, may be the custom API of the resource is not available.
	additionalErr := ""
	if err != nil {
		additionalErr = fmt.Sprintf(", additional error: %s", err.Error())
	}
	err = fmt.Errorf("apiVersion '%s', kind '%s' is not supported by cluster%s", apiVersion, kind, additionalErr)
	return nil, err
}

// GroupVersionResource returns a GroupVersionResource object to use with dynamic informer.
//
// This method is borrowed from kubectl and kubedog. The difference are:
// - lower case comparison with kind, name and all short names
func (c *kubernetesClient) GroupVersionResource(apiVersion string, kind string) (gvr schema.GroupVersionResource, err error) {
	apiRes, err := c.APIResource(apiVersion, kind)
	if err != nil {
		return
	}

	return schema.GroupVersionResource{
		Resource: apiRes.Name,
		Group:    apiRes.Group,
		Version:  apiRes.Version,
	}, nil

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
