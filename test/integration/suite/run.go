//go:build integration
// +build integration

package suite

import (
	"os"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/shell-operator/pkg/kube/dedupclient"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ClusterName   string
	ContextName   string
	KubeClient    *dedupclient.Client
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
	var err error
	KubeClient, err = dedupclient.New(dedupclient.Config{Context: ContextName}, log.NewNop())
	Expect(err).ShouldNot(HaveOccurred())

	ObjectPatcher = objectpatch.NewObjectPatcher(KubeClient, log.NewNop())
})
