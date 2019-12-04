// +build integration

package kubeclient_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/test/utils"
)

var ClusterName = "kube-client-test"
var ConfigPath string

func TestSuite(t *testing.T) {
	clusterVer := KindClusterVersion()
	if clusterVer != "" {
		ClusterName = fmt.Sprintf("%s-%s", ClusterName, clusterVer)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "kube client suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	Ω(KindCreateCluster(ClusterName)).Should(Succeed())
	fmt.Printf("Use kind flavour of k8s cluster v%s with node image %s\n", KindClusterVersion(), KindNodeImage())
	return []byte{}
}, func([]byte) {
	// Initialize kube client out-of-cluster
	ConfigPath = KindGetKubeconfigPath(ClusterName)
	Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: ConfigPath})).Should(Succeed())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(KindDeleteCluster(ClusterName)).Should(Succeed())
})
