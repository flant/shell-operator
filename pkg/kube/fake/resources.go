package fake

// set current kube-context to cluster with necessary version and run go generate
// it will create file with desired version and resources
//go:generate ./scripts/resources_generator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterVersion string

const (
	ClusterVersionV115 ClusterVersion = "v1.15.0"
	ClusterVersionV116 ClusterVersion = "v1.16.0"
	ClusterVersionV117 ClusterVersion = "v1.17.0"
	ClusterVersionV118 ClusterVersion = "v1.18.0"
	ClusterVersionV119 ClusterVersion = "v1.19.0"
	ClusterVersionV120 ClusterVersion = "v1.20.0"
)

func (cv ClusterVersion) String() string {
	return string(cv)
}

func (cv ClusterVersion) Major() string {
	return string(cv)[1:2]
}

func (cv ClusterVersion) Minor() string {
	return string(cv)[3:5]
}

func ClusterResources(version ClusterVersion) []*metav1.APIResourceList {
	switch version {
	case ClusterVersionV115:
		return v115ClusterResources

	case ClusterVersionV116:
		return v116ClusterResources

	case ClusterVersionV117:
		return v117ClusterResources

	case ClusterVersionV118:
		return v118ClusterResources

	case ClusterVersionV119:
		return v119ClusterResources

	case ClusterVersionV120:
		return v120ClusterResources
	}

	return nil
}
