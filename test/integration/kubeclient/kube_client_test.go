// +build integration

package kubeclient_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/test/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes API client package", func() {
	var clusterName = "kube-client-test"

	SynchronizedBeforeSuite(func() []byte {
		Ω(utils.KindCreateCluster(clusterName)).Should(Succeed())

		return []byte{}
	}, func([]byte) {
		// Initialize kube client out-of-cluster
		configPath := utils.KindGetKubeconfigPath(clusterName)
		Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: configPath})).Should(Succeed())
	})

	SynchronizedAfterSuite(func() {}, func() {
		Ω(utils.KindDeleteCluster(clusterName)).Should(Succeed())
	})

	When("client connect outside of the cluster", func() {

		It("should list deployments", func() {
			//// Initialize kube client for kube events hooks.
			//configPath := utils.KindGetKubeconfigPath(clusterName)
			//fmt.Printf("KUBECONFIG=%s", configPath)
			//Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: configPath})).Should(Succeed())

			list, err := kube.Kubernetes.AppsV1().Deployments("").List(metav1.ListOptions{})
			Ω(err).Should(Succeed())
			Ω(list.Items).Should(Not(HaveLen(0)))

			//for _, obj := range list.Items {
			//	t.Logf("%s/%s", obj.Namespace, obj.Name)
			//}

		})
		It("should find GroupVersionResource for Pod by kind", func() {
			gvr, err := kube.GroupVersionResourceByKind("Pod")
			Ω(err).Should(Succeed())
			Ω(gvr.Resource).Should(Equal("pods"))
			Ω(gvr.Group).Should(Equal(""))
			Ω(gvr.Version).Should(Equal("v1"))
		})
	})

})
