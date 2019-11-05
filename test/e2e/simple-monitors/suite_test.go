// +build e2e

package simple_monitors_test

import (
	"os"
	"testing"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/test/utils"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "monitor pods suite")
}

var ClusterName = "simple-monitors-test"
var ConfigPath string
var CurrentDir string

var _ = SynchronizedBeforeSuite(func() (res []byte) {
	Ω(KindCreateCluster(ClusterName)).Should(Succeed())
	return
}, func([]byte) {
	// Initialize kube client out-of-cluster
	ConfigPath = KindGetKubeconfigPath(ClusterName)
	Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: ConfigPath})).Should(Succeed())
	CurrentDir, _ = os.Getwd()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(KindDeleteCluster(ClusterName)).Should(Succeed())
})
