//go:build integration
// +build integration

package suite

import (
	"os"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	klient "github.com/flant/kube-client/client"
	objectpatch "github.com/flant/shell-operator/pkg/object_patch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	ClusterName   string
	ContextName   string
	KubeClient    *klient.Client
	ObjectPatcher *objectpatch.ObjectPatcher
)

func RunIntegrationSuite(t *testing.T, description string, clusterPrefix string) {
	ClusterName = os.Getenv("CLUSTER_NAME")
	ContextName = "kind-" + ClusterName

	RegisterFailHandler(Fail)
	RunSpecs(t, description)
}

var _ = BeforeSuite(func() {
	// Initialize kube client out-of-cluster
	KubeClient = klient.New(klient.WithLogger(log.NewNop()))
	KubeClient.WithContextName(ContextName)
	err := KubeClient.Init()
	Expect(err).ShouldNot(HaveOccurred())

	ObjectPatcher = objectpatch.NewObjectPatcher(KubeClient, log.NewNop())
})
