package kube

// TODO do not copy, use import "github.com/flant/kubedog/pkg/kube"

import (
	"fmt"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"

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

	Init() error

	DefaultNamespace() string
	Dynamic() dynamic.Interface

	GroupVersionResource(apiVersion string, kind string) (schema.GroupVersionResource, error)
	GroupVersionResourceByKind(kind string) (schema.GroupVersionResource, error)
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
}

func (c *kubernetesClient) WithContextName(name string) {
	c.contextName = name
}

func (c *kubernetesClient) WithConfigPath(path string) {
	c.configPath = path
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
	var defaultNs = ""

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
			}
		}
		configType = "in-cluster"
	}

	c.defaultNamespace = defaultNs

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

func (c *kubernetesClient) GroupVersionResource(apiVersion string, kind string) (schema.GroupVersionResource, error) {
	if apiVersion == "" {
		return c.GroupVersionResourceByKind(kind)
	}

	// Get only desired group
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion '%s' is invalid", apiVersion)
	}

	list, err := c.Discovery().ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion '%s' has no supported resources in cluster: %v", apiVersion, err)
	}

	var gvrForKind *schema.GroupVersionResource
	for _, resource := range list.APIResources {
		if len(resource.Verbs) == 0 {
			continue
		}

		// Debug mode will list all available CRDs for apiVersion
		log.Debugf("GVR: %30s %30s %30s", gv.String(), resource.Kind,
			fmt.Sprintf("%+v", append([]string{resource.Name}, resource.ShortNames...)),
		)

		if gvrForKind == nil && equalToOneOf(kind, append(resource.ShortNames, resource.Kind, resource.Name)...) {
			gvrForKind = &schema.GroupVersionResource{
				Resource: resource.Name,
				Group:    gv.Group,
				Version:  gv.Version,
			}
		}
	}

	if gvrForKind == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion '%s', kind '%s' is not supported by cluster", apiVersion, kind)
	}

	return *gvrForKind, nil
}

// GroupVersionResourceByKind returns GroupVersionResource object to use with dynamic informer.
//
// This method is borrowed from kubectl and kubedog. The difference are:
// - comparison with kind, name and shortnames
// - debug messages
func (c *kubernetesClient) GroupVersionResourceByKind(kind string) (schema.GroupVersionResource, error) {
	lists, err := c.Discovery().ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	var gvrForKind *schema.GroupVersionResource

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}

			// Debug mode will list all available CRDs
			log.Debugf("GVR: %30s %30s %30s", gv.String(), resource.Kind,
				fmt.Sprintf("%+v", append([]string{resource.Name}, resource.ShortNames...)),
			)

			if gvrForKind == nil && equalToOneOf(kind, append(resource.ShortNames, resource.Kind, resource.Name)...) {
				gvrForKind = &schema.GroupVersionResource{
					Resource: resource.Name,
					Group:    gv.Group,
					Version:  gv.Version,
				}
			}
		}
	}

	if gvrForKind == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("kind %s is not supported by cluster", kind)
	}

	return *gvrForKind, nil
}

func equalToOneOf(term string, choices ...string) bool {
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
