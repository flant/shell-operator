// +build integration

package kubeclient_test

import (
	"context"
	"testing"

	. "github.com/flant/shell-operator/test/integration/suite"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test(t *testing.T) {
	RunIntegrationSuite(t, "kube client suite", "kube-client-test")
}

var _ = Describe("Kubernetes API client package", func() {
	When("client connect outside of the cluster", func() {

		It("should list deployments", func() {
			list, err := KubeClient.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
			Ω(err).Should(Succeed())
			Ω(list.Items).Should(Not(HaveLen(0)))
		})

		It("should find GroupVersionResource for Pod by kind only", func() {
			gvr, err := KubeClient.GroupVersionResource("", "Pod")
			Ω(err).Should(Succeed())
			Ω(gvr.Resource).Should(Equal("pods"))
			Ω(gvr.Group).Should(Equal(""))
			Ω(gvr.Version).Should(Equal("v1"))
		})

		It("should find GroupVersionResource for Deployment by apiVersion and kind", func() {
			gvr, err := KubeClient.GroupVersionResource("apps/v1", "Deployment")
			Ω(err).Should(Succeed())
			Ω(gvr.Resource).Should(Equal("deployments"))
			Ω(gvr.Group).Should(Equal("apps"))
			Ω(gvr.Version).Should(Equal("v1"))
		})
	})
})
