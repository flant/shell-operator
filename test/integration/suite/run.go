//go:build integration
// +build integration

package suite

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/log"
	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

var (
	ClusterName   string
	ContextName   string
	KubeClient    *klient.Client
	ObjectPatcher *object_patch.ObjectPatcher
)

func RunIntegrationSuite(t *testing.T, description string, clusterPrefix string) {
	ClusterName = os.Getenv("CLUSTER_NAME")
	ContextName = "kind-" + ClusterName

	RegisterFailHandler(Fail)
	RunSpecs(t, description)
}

var _ = BeforeSuite(func() {
	// Initialize kube client out-of-cluster
	KubeClient = klient.New()
	KubeClient.WithContextName(ContextName)
	err := KubeClient.Init()
	Expect(err).ShouldNot(HaveOccurred())

	ObjectPatcher = object_patch.NewObjectPatcher(KubeClient, log.NewNop())
})
