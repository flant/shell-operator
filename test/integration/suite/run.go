// +build integration

package suite

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/test/utils"

	"github.com/flant/shell-operator/pkg/kube"
)

var ClusterName string
var ConfigPath string

func RunIntegrationSuite(t *testing.T, description string, clusterPrefix string) {
	ClusterName = KindClusterName(clusterPrefix)

	RegisterFailHandler(Fail)
	RunSpecs(t, description)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	Ω(KindCreateCluster(ClusterName)).Should(Succeed())
	fmt.Println(KindUseClusterMessage(ClusterName))
	return []byte{}
}, func([]byte) {
	// Initialize kube client out-of-cluster
	ConfigPath = KindGetKubeconfigPath(ClusterName)
	Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: ConfigPath})).Should(Succeed())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(KindDeleteCluster(ClusterName)).Should(Succeed())
})
