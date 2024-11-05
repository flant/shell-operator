//go:build integration
// +build integration

package kube_event_manager_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	. "github.com/flant/shell-operator/test/integration/suite"
	testutils "github.com/flant/shell-operator/test/utils"
)

func Test(t *testing.T) {
	RunIntegrationSuite(t, "KubeEventManager suite", "kube-event-manager-test")
}

var _ = Describe("Binding 'kubernetes' with kind 'Pod' should emit KubeEvent objects", func() {
	var KubeEventsManager kube_events_manager.KubeEventsManager

	BeforeEach(func() {
		KubeEventsManager = kube_events_manager.NewKubeEventsManager(context.Background(), KubeClient, log.NewNop())
	})

	Context("with configVersion: v1", func() {
		var monitorConfig *kube_events_manager.MonitorConfig

		BeforeEach(func() {
			monitorConfig = &kube_events_manager.MonitorConfig{
				Kind:                    "Pod",
				ApiVersion:              "v1",
				KeepFullObjectsInMemory: true,
				EventTypes: []WatchEventType{
					WatchEventAdded,
				},
				NamespaceSelector: &NamespaceSelector{
					NameSelector: &NameSelector{
						MatchNames: []string{"default"},
					},
				},
			}
			monitorConfig.Metadata.MonitorId = "test-abcd"

			err := KubeEventsManager.AddMonitor(monitorConfig)
			Expect(err).ShouldNot(HaveOccurred())
			KubeEventsManager.StartMonitor(monitorConfig.Metadata.MonitorId)
		})

		It("should have cached objects", func(done Done) {
			Expect(KubeEventsManager.HasMonitor(monitorConfig.Metadata.MonitorId)).Should(BeTrue())

			m := KubeEventsManager.GetMonitor(monitorConfig.Metadata.MonitorId)
			Expect(m).ShouldNot(BeNil())
			snapshot := m.Snapshot()
			Expect(snapshot).ShouldNot(BeNil())
			Expect(snapshot).Should(HaveLen(0), "No pods in default namespace. Snapshot at start should have no objects.")

			close(done)
		}, 10)

		When("Pod is Added", func() {
			JustBeforeEach(func() {
				app.SetupLogging(nil, log.NewNop())

				// Unlock KubeEvent emitting.
				m := KubeEventsManager.GetMonitor(monitorConfig.Metadata.MonitorId)
				m.EnableKubeEventCb()

				testutils.Kubectl(ContextName).Apply("default", "testdata/test-pod.yaml")
			})

			It("should return KubeEvent with type 'Event'", func() {
				ev := <-KubeEventsManager.Ch()

				fmt.Fprintf(GinkgoWriter, "Receive %#v\n", ev)

				Expect(ev.MonitorId).To(Equal(monitorConfig.Metadata.MonitorId))
				Expect(string(ev.Type)).To(Equal("Event"))

				Expect(ev.Objects).ShouldNot(BeNil())
				Expect(ev.Objects).Should(HaveLen(1))
				Expect(ev.Objects[0].Object.GetName()).Should(Equal("test"))
				Expect(ev.Objects[0].FilterResult).Should(BeNil(), "filterResult should be empty if monitor has no jqFilter or filterFunc")
			}, 25)

			AfterEach(func() {
				testutils.Kubectl(ContextName).Delete("default", "pod/test")
			})
		})
	})
})
