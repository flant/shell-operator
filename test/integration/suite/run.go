// +build integration

package suite

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	. "github.com/flant/shell-operator/test/utils"
)

var (
	ClusterName   string
	ContextName   string
	KubeClient    klient.Client
	ObjectPatcher *object_patch.ObjectPatcher
)

func RunIntegrationSuite(t *testing.T, description string, clusterPrefix string) {
	ClusterName = KindClusterName(clusterPrefix)

	RegisterFailHandler(Fail)
	RunSpecs(t, description)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	Expect(KindCreateCluster(ClusterName)).Should(Succeed())
	fmt.Println(KindUseClusterMessage(ClusterName))
	return []byte{}
}, func([]byte) {
	// Initialize kube client out-of-cluster
	ContextName = KindGetKubeContext(ClusterName)
	KubeClient = klient.New()
	KubeClient.WithContextName(ContextName)
	err := KubeClient.Init()
	Expect(err).ShouldNot(HaveOccurred())

	ObjectPatcher = object_patch.NewObjectPatcher(KubeClient)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Expect(KindDeleteCluster(ClusterName)).Should(Succeed())
})
