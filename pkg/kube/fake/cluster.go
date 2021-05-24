package fake

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/utils/manifest"
)

type FakeCluster struct {
	KubeClient kube.KubernetesClient

	Discovery *fakediscovery.FakeDiscovery
}

func NewFakeCluster(ver ClusterVersion) *FakeCluster {
	if ver == "" {
		ver = ClusterVersionV119
	}
	cres := ClusterResources(ver)

	// FIXME: below code will be used in go-client 0.20.x pass it to NewFakeKubernetesClient
	// gvrToListKind := make(map[schema.GroupVersionResource]string)
	// for _, gr := range cres {
	// 	for _, res := range gr.APIResources {
	// 		gvr := schema.GroupVersionResource{
	// 			Group:    res.Group,
	// 			Version:  res.Version,
	// 			Resource: res.Name,
	// 		}
	// 		gvrToListKind[gvr] = res.Kind + "List"
	// 	}
	// }

	fc := &FakeCluster{}
	fc.KubeClient = kube.NewFakeKubernetesClient(nil)

	var ok bool
	fc.Discovery, ok = fc.KubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		panic("couldn't convert Discovery() to *FakeDiscovery")
	}
	fc.Discovery.FakedServerVersion = &version.Info{GitCommit: ver.String(), Major: ver.Major(), Minor: ver.Minor()}
	fc.Discovery.Resources = cres

	return fc
}

func (fc *FakeCluster) CreateNs(ns string) {
	nsObj := &corev1.Namespace{}
	nsObj.Name = ns
	_, _ = fc.KubeClient.CoreV1().Namespaces().Create(context.TODO(), nsObj, metav1.CreateOptions{})
}

// RegisterCRD registers custom resources for the cluster
func (fc *FakeCluster) RegisterCRD(group, version, kind string, namespaced bool) {
	scheme.Scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: group, Version: version, Kind: kind}, &unstructured.Unstructured{})
	newResource := metav1.APIResource{
		Kind:       kind,
		Name:       Pluralize(kind),
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Version:    version,
		Namespaced: namespaced,
	}
	for _, resource := range fc.Discovery.Resources {
		if resource.GroupVersion == group+"/"+version {
			resource.APIResources = append(resource.APIResources, newResource)
			return
		}
	}
	fc.Discovery.Resources = append(fc.Discovery.Resources, &metav1.APIResourceList{
		GroupVersion: group + "/" + version,
		APIResources: []metav1.APIResource{newResource},
	})
}

func (fc *FakeCluster) FindGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	gvr := findGvr(fc.Discovery.Resources, apiVersion, kind)
	if gvr == nil {
		return nil, fmt.Errorf("GVR for %s is not find", kind)
	}
	return gvr, nil
}

func (fc *FakeCluster) MustFindGVR(apiVersion, kind string) *schema.GroupVersionResource {
	return findGvr(fc.Discovery.Resources, apiVersion, kind)
}

func (fc *FakeCluster) CreateSimpleNamespaced(ns string, kind string, name string) {
	fc.CreateNs(ns)

	gvr := fc.MustFindGVR("", kind)
	obj := manifest.NewManifest(gvr.GroupVersion().String(), kind, name).ToUnstructured()

	_, err := fc.KubeClient.Dynamic().Resource(*gvr).Namespace(ns).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (fc *FakeCluster) DeleteSimpleNamespaced(ns string, kind string, name string) {
	gvr := fc.MustFindGVR("", kind)
	err := fc.KubeClient.Dynamic().Resource(*gvr).Namespace(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		panic(err)
	}
}

func (fc *FakeCluster) Create(ns string, m manifest.Manifest) error {
	gvr, err := fc.FindGVR(m.ApiVersion(), m.Kind())
	if err != nil {
		return err
	}
	_, err = fc.KubeClient.Dynamic().Resource(*gvr).Namespace(m.Namespace(ns)).Create(context.TODO(), m.ToUnstructured(), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating object failed: %v", err)
	}
	return nil
}

func (fc *FakeCluster) Delete(ns string, m manifest.Manifest) error {
	gvr, err := fc.FindGVR(m.ApiVersion(), m.Kind())
	if err != nil {
		return err
	}

	err = fc.KubeClient.Dynamic().Resource(*gvr).Namespace(m.Namespace(ns)).Delete(context.TODO(), m.Name(), metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting object failed: %v", err)
	}
	return nil
}

func (fc *FakeCluster) Update(ns string, m manifest.Manifest) error {
	gvr, err := fc.FindGVR(m.ApiVersion(), m.Kind())
	if err != nil {
		return err
	}

	_, err = fc.KubeClient.Dynamic().Resource(*gvr).Namespace(m.Namespace(ns)).Update(context.TODO(), m.ToUnstructured(), metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating object failed: %v", err)
	}
	return nil
}

func findGvr(resources []*metav1.APIResourceList, apiVersion, kindOrName string) *schema.GroupVersionResource {
	for _, apiResourceGroup := range resources {
		if apiVersion != "" && apiResourceGroup.GroupVersion != apiVersion {
			continue
		}
		for _, apiResource := range apiResourceGroup.APIResources {
			if strings.EqualFold(apiResource.Kind, kindOrName) || strings.EqualFold(apiResource.Name, kindOrName) {
				// ignore parse error, because FakeClusterResources should be valid
				gv, _ := schema.ParseGroupVersion(apiResourceGroup.GroupVersion)
				return &schema.GroupVersionResource{
					Resource: apiResource.Name,
					Group:    gv.Group,
					Version:  gv.Version,
				}
			}
		}
	}
	return nil
}

// Pluralize is the simplest way to make a plural form (like resource) from k8s object Kind
// ex: User -> users
//     Prometheus -> prometheuses
//     NetworkPolicy -> netwrokpolicies
//     CustomPrometheusRules -> customprometheusrules
func Pluralize(kind string) string {
	if kind == "" {
		return kind
	}

	kind = strings.ToLower(kind)

	// maybe we dont need more complex pluralizer here
	// but if we do, can take smth like https://github.com/gertd/go-pluralize
	if strings.HasSuffix(kind, "es") {
		return kind
	} else if strings.HasSuffix(kind, "s") {
		return kind + "es"
	} else if strings.HasSuffix(kind, "cy") {
		return strings.TrimSuffix(kind, "y") + "ies"
	}

	return kind + "s"
}
