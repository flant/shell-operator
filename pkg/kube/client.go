package kube

// TODO do not copy, use import "github.com/flant/kubedog/pkg/kube"

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/romana/rlog"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (only required to authenticate against GKE clusters)
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

const (
	kubeTokenFilePath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	kubeNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

var (
	Kubernetes       kubernetes.Interface
	DynamicClient    dynamic.Interface
	DefaultNamespace string
	Context          string
)

type InitOptions struct {
	KubeContext string
	KubeConfig  string
}

func Init(opts InitOptions) error {
	rlog.Info("KUBE Init Kubernetes client")

	var err error
	var config *rest.Config

	// Try to load from kubeconfig in flags or from ~/.kube/config
	config, outOfClusterErr := getOutOfClusterConfig(opts.KubeContext, opts.KubeConfig)

	if config == nil {
		if hasInClusterConfig() {
			// Try to configure as inCluster
			config, err = getInClusterConfig()
			if err != nil {
				if opts.KubeConfig != "" || opts.KubeContext != "" {
					if outOfClusterErr != nil {
						err = fmt.Errorf("out-of-cluster config error: %v, in-cluster config error: %v", outOfClusterErr, err)
						rlog.Errorf("KUBE-INIT Kubernetes client problems: %s", err)
						return err
					}
				} else {
					rlog.Errorf("KUBE-INIT Kubernetes client in-cluster problem: %s", err)
					return err
				}
			}
		} else {
			// if not in cluster return outOfCluster error
			if outOfClusterErr != nil {
				rlog.Errorf("KUBE-INIT Kubernetes client out-of-cluster problem: %s", outOfClusterErr)
				return outOfClusterErr
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		rlog.Errorf("KUBE-INIT Kubernetes client configuration problem: %s", err)
		return err
	}
	Kubernetes = clientset

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	DynamicClient = dynamicClient

	rlog.Info("KUBE-INIT Kubernetes client is configured successfully")

	return nil
}

type GetClientsOptions struct {
	KubeConfig string
}

func GetAllClients(opts GetClientsOptions) ([]kubernetes.Interface, error) {
	// Try to load contexts from kubeconfig in flags or from ~/.kube/config
	var outOfClusterErr error
	contexts, outOfClusterErr := getOutOfClusterContexts(opts.KubeConfig)
	// return if contexts are loaded successfully
	if contexts != nil {
		return contexts, nil
	}
	if hasInClusterConfig() {
		context, err := getInClusterContext()
		if err != nil {
			return nil, err
		}
		return []kubernetes.Interface{context}, nil
	}
	// if not in cluster return outOfCluster error
	if outOfClusterErr != nil {
		return nil, outOfClusterErr
	}

	return nil, nil
}

func makeOutOfClusterClientConfigError(kubeConfig, kubeContext string, err error) error {
	baseErrMsg := fmt.Sprintf("out-of-cluster configuration problem")

	if kubeConfig != "" {
		baseErrMsg += fmt.Sprintf(", custom kube config path is %q", kubeConfig)
	}

	if kubeContext != "" {
		baseErrMsg += fmt.Sprintf(", custom kube context is %q", kubeContext)
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

func getOutOfClusterConfig(contextName string, configPath string) (config *rest.Config, err error) {
	clientConfig := getClientConfig(contextName, configPath)

	ns, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, fmt.Errorf("cannot determine default kubernetes namespace: %s", err)
	}
	DefaultNamespace = ns

	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, makeOutOfClusterClientConfigError(configPath, contextName, err)
	}

	rc, err := clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot get raw kubernetes config: %s", err)
	}

	if contextName != "" {
		Context = contextName
	} else {
		Context = rc.CurrentContext
	}

	return
}

func getOutOfClusterContexts(configPath string) (contexts []kubernetes.Interface, err error) {
	contexts = make([]kubernetes.Interface, 0)

	rc, err := getClientConfig("", configPath).RawConfig()
	if err != nil {
		return nil, err
	}

	for contextName := range rc.Contexts {
		clientConfig := getClientConfig(contextName, configPath)

		config, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, makeOutOfClusterClientConfigError(configPath, contextName, err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		contexts = append(contexts, clientset)
	}

	return
}

func getInClusterConfig() (config *rest.Config, err error) {
	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster configuration problem: %s", err)
	}

	data, err := ioutil.ReadFile(kubeNamespaceFilePath)
	if err != nil {
		return nil, fmt.Errorf("in-cluster configuration problem: cannot determine default kubernetes namespace: error reading %s: %s", kubeNamespaceFilePath, err)
	}
	DefaultNamespace = string(data)

	return
}

func getInClusterContext() (context kubernetes.Interface, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster configuration problem: %s", err)
	}

	context, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return
}

func GroupVersionResource(apiVersion string, kind string) (schema.GroupVersionResource, error) {
	if apiVersion == "" {
		return GroupVersionResourceByKind(kind)
	}

	// Get only desired group
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion '%s' is invalid", apiVersion)
	}

	list, err := Kubernetes.Discovery().ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion '%s' has no supported resources in cluster: %v", apiVersion, err)
	}

	var gvrForKind *schema.GroupVersionResource
	for _, resource := range list.APIResources {
		if len(resource.Verbs) == 0 {
			continue
		}

		// Debug mode will list all available CRDs for apiVersion
		rlog.Debugf("GVR: %30s %30s %30s", gv.String(), resource.Kind,
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
func GroupVersionResourceByKind(kind string) (schema.GroupVersionResource, error) {
	lists, err := Kubernetes.Discovery().ServerPreferredResources()
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
			rlog.Debugf("GVR: %30s %30s %30s", gv.String(), resource.Kind,
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
