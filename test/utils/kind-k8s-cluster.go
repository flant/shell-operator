// +build test

package utils

import (
	"fmt"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/create"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/globals"
)

func KindCreateCluster(clusterName string) error {
	// Check if the cluster name already exists
	known, err := cluster.IsKnown(clusterName)
	if err != nil {
		return err
	}
	if known {
		return fmt.Errorf("a cluster with the name %q already exists", clusterName)
	}

	// create a cluster context and create the cluster
	ctx := cluster.NewContext(clusterName)
	fmt.Printf("Creating cluster %q ...\n", clusterName)
	if err = ctx.Create(
		create.WithConfigFile(""),
		create.WithNodeImage("kindest/node:v1.13.10"),
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

func KindDeleteCluster(clusterName string) error {
	// Delete the cluster
	fmt.Printf("Deleting cluster %q ...\n", clusterName)
	ctx := cluster.NewContext(clusterName)
	if err := ctx.Delete(); err != nil {
		return errors.Wrap(err, "failed to delete cluster")
	}
	return nil
}

func KindGetKubeconfigPath(clusterName string) string {
	return cluster.NewContext(clusterName).KubeConfigPath()
}
