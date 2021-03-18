// +build test

package utils

import (
	"fmt"
	"os"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
)

// KindCreateCluster acts as a kind create command
func KindCreateCluster(clusterName string) error {
	logger := cmd.NewLogger()
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
	)

	// Check if the cluster name already exists
	n, err := provider.ListNodes(clusterName)
	if err != nil {
		return err
	}
	if len(n) != 0 {
		if os.Getenv("KIND_USE_CLUSTER") == "" {
			return fmt.Errorf("node(s) already exist for a cluster with the name '%s'", clusterName)
		} else {
			// Cluster is already exists and user ask to use it.
			return nil
		}
	}

	// create a cluster context and create the cluster
	fmt.Printf("KIND: Creating cluster '%s' ...\n", clusterName)
	if err = provider.Create(
		clusterName,
		cluster.CreateWithNodeImage(KindNodeImage()),
		cluster.CreateWithRetain(false),
		cluster.CreateWithWaitForReady(600*time.Second),
		cluster.CreateWithKubeconfigPath(""),
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	); err != nil {
		if errs := errors.Errors(err); errs != nil {
			for _, problem := range errs {
				_, _ = fmt.Fprintf(os.Stderr, "KIND: %v", problem)
			}
			return errors.New("aborting due to invalid configuration")
		}
		return errors.Wrap(err, "failed to create cluster")
	}

	n, err = provider.ListNodes(clusterName)
	if err != nil {
		return err
	}
	if len(n) == 0 {
		return fmt.Errorf("no kind nodes for created cluster '%s'", clusterName)
	}
	for _, node := range n {
		fmt.Printf("%s ", node.String())
	}

	return nil
}

func KindDeleteCluster(clusterName string) error {
	if os.Getenv("KIND_USE_CLUSTER") != "" {
		return nil
	}
	// Delete the cluster
	fmt.Printf("Deleting cluster '%s' ...\n", clusterName)

	logger := cmd.NewLogger()
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
	)

	if err := provider.Delete(clusterName, ""); err != nil {
		return errors.Wrapf(err, "failed to delete cluster '%s'", clusterName)
	}
	return nil
}

func KindGetKubeContext(clusterName string) string {
	return "kind-" + clusterName
}

func KindClusterVersion() string {
	k8sVer := os.Getenv("KIND_CLUSTER_VERSION")
	if k8sVer == "" {
		k8sVer = "1.20"
	}

	return k8sVer
}

// KindNodeImage maps cluster Major.Minor version to image name
//
// See: https://hub.docker.com/r/kindest/node/tags
func KindNodeImage() string {
	image := os.Getenv("KIND_IMAGE")
	if image != "" {
		return image
	}
	images := map[string]string{
		"1.14": "kindest/node:v1.14.10",
		"1.15": "kindest/node:v1.15.12",
		"1.16": "kindest/node:v1.16.15",
		"1.17": "kindest/node:v1.17.17",
		"1.18": "kindest/node:v1.18.15",
		"1.19": "kindest/node:v1.19.7",
		"1.20": "kindest/node:v1.20.2",
	}

	return images[KindClusterVersion()]
}

func KindClusterName(clusterPrefix string) string {
	name := os.Getenv("KIND_CLUSTER_NAME")
	if name != "" {
		return name
	}

	forceName := os.Getenv("KIND_USE_CLUSTER")
	if forceName != "" {
		return forceName
	}

	clusterVer := KindClusterVersion()
	if clusterVer != "" {
		return fmt.Sprintf("%s-%s", clusterPrefix, clusterVer)
	} else {
		return clusterPrefix
	}
}

func KindUseClusterMessage(clusterName string) string {
	forceName := os.Getenv("KIND_USE_CLUSTER")
	if forceName != "" {
		return fmt.Sprintf("Use 'kind' flavour of k8s cluster with name '%s'", clusterName)
	}
	return fmt.Sprintf("Use 'kind' flavour of k8s cluster v%s with node image %s", KindClusterVersion(), KindNodeImage())

}
