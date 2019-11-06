// +build integration

package kubeclient_test

import (
	"fmt"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	. "github.com/flant/shell-operator/test/utils"
)

var ClusterName = "kube-client-test"

func TestSuite(t *testing.T) {
	clusterVer := KindClusterVersion()
	if clusterVer != "" {
		ClusterName = fmt.Sprintf("%s-%s", ClusterName, clusterVer)
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "kube client suite")
}
