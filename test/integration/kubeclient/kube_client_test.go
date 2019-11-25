// +build integration

package kubeclient_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/kube"
)

var _ = Describe("Kubernetes API client package", func() {
	When("client connect outside of the cluster", func() {

		It("should list deployments", func() {
			list, err := kube.Kubernetes.AppsV1().Deployments("").List(metav1.ListOptions{})
			Ω(err).Should(Succeed())
			Ω(list.Items).Should(Not(HaveLen(0)))
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
