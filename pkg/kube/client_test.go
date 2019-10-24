package kube

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/create"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/globals"
)

func Test_Discover(t *testing.T) {
	// Test works only with real cluster.
	t.SkipNow()

	err := Init(InitOptions{})
	if err != nil {
		t.Error(err)
	}

	lists, err := Kubernetes.Discovery().ServerPreferredResources()
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("Got %d lists\n", len(lists))

	for _, list := range lists {
		fmt.Printf("\n\n")
		fmt.Printf("%s has %d resources\n", list.GroupVersion, len(list.APIResources))

		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			t.Error(err)
			continue
		}

		for _, resource := range list.APIResources {
			gvr := schema.GroupVersionResource{
				Resource: resource.Name,
				Group:    gv.Group,
				Version:  gv.Version,
			}

			fmt.Printf("%30s %30s %30s %d:%+v\n",
				gvr.GroupVersion().String(),
				resource.Kind,
				fmt.Sprintf("%+v", append([]string{resource.Name}, resource.ShortNames...)),
				len(resource.Verbs), resource.Verbs)
		}
	}
}

var clName = "test-cluster"

func Test_ListDeployments(t *testing.T) {
	var err error

	err = createCluster(clName)
	if err != nil {
		t.Logf("create cluster %s: %s", clName, err)
		return
	}

	// Initialize kube client for kube events hooks.
	configPath := kubeconfigPath(clName)
	fmt.Printf("KUBECONFIG=%s", configPath)
	err = Init(InitOptions{KubeContext: "", KubeConfig: configPath})
	if err != nil {
		t.Logf("Fatal: initialize kube client: %s", err)
		return
	}

	list, err := Kubernetes.AppsV1().Deployments("").List(metav1.ListOptions{})
	if err != nil {
		t.Logf("list deployments: %s", err)
	}

	for _, obj := range list.Items {
		t.Logf("%s/%s", obj.Namespace, obj.Name)
	}

	err = deleteCluster(clName)
	if err != nil {
		t.Logf("delete cluster %s: %s", clName, err)
	}
}

func createCluster(name string) error {
	// Check if the cluster name already exists
	known, err := cluster.IsKnown(clName)
	if err != nil {
		return err
	}
	if known {
		return fmt.Errorf("a cluster with the name %q already exists", name)
	}

	// create a cluster context and create the cluster
	ctx := cluster.NewContext(name)
	fmt.Printf("Creating cluster %q ...\n", name)
	if err = ctx.Create(
		create.WithConfigFile(""),
		create.WithNodeImage("kindest/node:v1.15"),
		//create.Retain(flags.Retain),
		create.WaitForReady(time.Second*300),
	); err != nil {
		if errs := errors.Errors(err); errs != nil {
			for _, problem := range errs {
				globals.GetLogger().Errorf("%v", problem)
			}
			return errors.New("aborting due to invalid configuration")
		}
		return errors.Wrap(err, "failed to create cluster")
	}

	return nil

}

func deleteCluster(name string) error {
	// Delete the cluster
	fmt.Printf("Deleting cluster %q ...\n", name)
	ctx := cluster.NewContext(name)
	if err := ctx.Delete(); err != nil {
		return errors.Wrap(err, "failed to delete cluster")
	}
	return nil
}

func kubeconfigPath(name string) string {
	return cluster.NewContext(name).KubeConfigPath()
}
