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
var ContextName string
var KubeClient kube.KubernetesClient

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
	ContextName = KindGetKubeContext(ClusterName)
	KubeClient = kube.NewKubernetesClient()
	KubeClient.WithContextName(ContextName)
	err := KubeClient.Init()
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(KindDeleteCluster(ClusterName)).Should(Succeed())
})
