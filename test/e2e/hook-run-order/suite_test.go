// +build e2e

package simple_monitors_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/test/utils"
)

var ClusterName = "test-hook-run-order"
var ConfigPath string
var CurrentDir string

func TestSuite(t *testing.T) {

	clusterVer := KindClusterVersion()
	if clusterVer != "" {
		ClusterName = fmt.Sprintf("%s-%s", ClusterName, clusterVer)
	}

	CurrentDir, _ = os.Getwd()

	RegisterFailHandler(Fail)
	RunSpecs(t, "hook run order")
}

var _ = SynchronizedBeforeSuite(func() (res []byte) {
	Ω(KindCreateCluster(ClusterName)).Should(Succeed())
	fmt.Printf("Use kind flavour of k8s cluster v%s with node image %s\n", KindClusterVersion(), KindNodeImage())
	return
}, func([]byte) {
	// Initialize kube client out-of-cluster
	ConfigPath = KindGetKubeconfigPath(ClusterName)
	Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: ConfigPath})).Should(Succeed())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(KindDeleteCluster(ClusterName)).Should(Succeed())
})
