// +build integration

package kube_event_manager_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/flant/shell-operator/test/integration/suite"
	. "github.com/flant/shell-operator/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

func Test(t *testing.T) {
	RunIntegrationSuite(t, "KubeEventManager suite", "kube-event-manager-test")
}

var _ = Describe("Binding 'kubernetes' with kind 'Pod' should emit KubeEvent objects", func() {
	var KubeEventsManager kube_events_manager.KubeEventsManager

	BeforeEach(func() {
		KubeEventsManager = kube_events_manager.NewKubeEventsManager()
		KubeEventsManager.WithContext(context.Background())
		KubeEventsManager.WithKubeClient(KubeClient)
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
			Ω(err).ShouldNot(HaveOccurred())
			KubeEventsManager.StartMonitor(monitorConfig.Metadata.MonitorId)
		})

		It("should have cached objects", func(done Done) {
			Expect(KubeEventsManager.HasMonitor(monitorConfig.Metadata.MonitorId)).Should(BeTrue())

			m := KubeEventsManager.GetMonitor(monitorConfig.Metadata.MonitorId)
			Expect(m).ShouldNot(BeNil())
			snapshot := m.Snapshot()
			Expect(snapshot).ShouldNot(BeNil())
			Expect(snapshot).Should(HaveLen(0), "No pods in default namespace. Snapshot at start should have no objects.")

			// Unlock KubeEvent emitting.
			m.EnableKubeEventCb()

			close(done)

		}, 6)

		When("Pod is Added", func() {

			JustBeforeEach(func() {
				app.SetupLogging()
				Kubectl(ContextName).Apply("default", "testdata/test-pod.yaml")
			})

			It("should return KubeEvent with type 'Event'", func(done Done) {
				ev := <-KubeEventsManager.Ch()

				fmt.Printf("%#v\n", ev)

				Ω(ev.MonitorId).To(Equal(monitorConfig.Metadata.MonitorId))
				Ω(string(ev.Type)).To(Equal("Event"))

				Ω(ev.Objects).ShouldNot(BeNil())
				Ω(ev.Objects).Should(HaveLen(1))
				Ω(ev.Objects[0].Object.GetName()).Should(Equal("test"))
				Ω(ev.Objects[0].FilterResult).Should(Equal(""))

				close(done)
			}, 25)

			AfterEach(func() {
				Kubectl(ContextName).Delete("default", "pod/test")
			})
		})
	})
})
