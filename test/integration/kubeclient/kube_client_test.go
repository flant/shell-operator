//go:build integration
// +build integration

package kubeclient_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/flant/shell-operator/test/integration/suite"
)

func Test(t *testing.T) {
	RunIntegrationSuite(t, "kube client suite", "kube-client-test")
}

var _ = Describe("Kubernetes API client package", func() {
	When("client connect outside of the cluster", func() {
		It("should list deployments", func(ctx context.Context) {
			list, err := KubeClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
			Ω(err).Should(Succeed())
			Ω(list.Items).Should(Not(HaveLen(0)))
		}, SpecTimeout(10*time.Second))

		It("should find GroupVersionResource for Pod by kind only", func(ctx context.Context) {
			gvr, err := KubeClient.GroupVersionResource("", "Pod")
			Ω(err).Should(Succeed())
			Ω(gvr.Resource).Should(Equal("pods"))
			Ω(gvr.Group).Should(Equal(""))
			Ω(gvr.Version).Should(Equal("v1"))
		}, SpecTimeout(10*time.Second))

		It("should find GroupVersionResource for Deployment by apiVersion and kind", func(ctx context.Context) {
			gvr, err := KubeClient.GroupVersionResource("apps/v1", "Deployment")
			Ω(err).Should(Succeed())
			Ω(gvr.Resource).Should(Equal("deployments"))
			Ω(gvr.Group).Should(Equal("apps"))
			Ω(gvr.Version).Should(Equal("v1"))
		}, SpecTimeout(10*time.Second))
	})
})
