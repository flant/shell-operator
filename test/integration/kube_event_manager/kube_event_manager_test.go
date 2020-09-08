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

//var verBcs map[string]string
//var _ = Ω(JqFilter(verBcs, `.[0] | has("objects")`)).To(Equal("true"), JqFilter(verBcs, `.`))
//var _ = Ω(verBcs).To(MatchJq(`.[0] | has("objects")`, Equal(true)))

func Test(t *testing.T) {
	RunIntegrationSuite(t, "KubeEventManager suite", "kube-event-manager-test")
}

var _ = Describe("Subscription to Pods should emit KubeEvent objects", func() {
	var KubeEventsManager kube_events_manager.KubeEventsManager

	BeforeEach(func() {
		KubeEventsManager = kube_events_manager.NewKubeEventsManager()
		KubeEventsManager.WithContext(context.Background())
		KubeEventsManager.WithKubeClient(KubeClient)
	})

	Context("with configVersion: v1", func() {
		var monitorConfig *kube_events_manager.MonitorConfig
		var ev *KubeEvent

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

			var err error
			ev, err = KubeEventsManager.AddMonitor(monitorConfig)
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Printf("ev: %#v\n", ev)
		})

		It("should return KubeEvent Synchronization on AddMonitor", func(done Done) {
			Ω(ev.MonitorId).To(Equal(monitorConfig.Metadata.MonitorId))

			Ω(ev.Objects).Should(HaveLen(0), "No pods in default namespace, synchronization should have no objects")

			//bcList := hook.ConvertKubeEventToBindingContext(*ev, "kubernetes")
			//verBcs := hook.ConvertBindingContextList("v1", bcList)

			//Ω(verBcs).To(MatchJq(`.[0] | has("objects")`, "true"))
			//Ω(verBcs).To(MatchJq(`.[0].objects | map(select(has("filterResult"))) | length`, "0"))

			close(done)

		}, 6)

		When("Pod is Added", func() {

			JustBeforeEach(func() {
				app.SetupLogging()
				Kubectl(ContextName).Apply("default", "testdata/test-pod.yaml")
			})

			It("should return KubeEvent with type 'Event'", func(done Done) {
				Ω(KubeEventsManager.HasMonitor(monitorConfig.Metadata.MonitorId)).To(BeTrue())
				KubeEventsManager.Start()

				ev := <-KubeEventsManager.Ch()

				fmt.Printf("%#v\n", ev)

				Ω(ev.MonitorId).To(Equal(monitorConfig.Metadata.MonitorId))
				Ω(string(ev.Type)).To(Equal("Event"))

				Ω(ev.Objects).ShouldNot(BeNil())
				Ω(ev.Objects).Should(HaveLen(1))
				Ω(ev.Objects[0].Object.GetName()).Should(Equal("test"))

				//Ω(ev.Object).ShouldNot(BeNil())
				//Ω(ev.FilterResult).Should(Equal(""))

				//bcList := kube_event.ConvertKubeEventToBindingContext(ev, "kubernetes")
				//verBcs := hook.ConvertBindingContextList("v1", bcList)

				//Ω(verBcs).To(MatchJq(`.[0] | has("objects")`, "false"))
				//Ω(verBcs).To(MatchJq(`.[0] | has("filterResult")`, "false"))

				//Ω(verBcs).To(MatchJq(`.[0] | length`, "4"))
				//Ω(verBcs).To(MatchJq(`.[0].binding`, `"kubernetes"`))
				//Ω(verBcs).To(MatchJq(`.[0].type`, `"Event"`))
				//Ω(verBcs).To(MatchJq(`.[0].watchEvent`, `"Added"`))

				//Ω(verBcs).To(MatchJq(`.[0] | has("object")`, `true`))
				//Ω(verBcs).To(MatchJq(`.[0].object.metadata.name`, `"test"`))

				close(done)
			}, 25)

			AfterEach(func() {
				Kubectl(ContextName).Delete("default", "pod/test")
			})
		})

		//It("should find GroupVersionResource for Pod by kind", func() {
		//	gvr, err := kube.GroupVersionResourceByKind("Pod")
		//	Ω(err).Should(Succeed())
		//	Ω(gvr.Resource).Should(Equal("pods"))
		//	Ω(gvr.Group).Should(Equal(""))
		//	Ω(gvr.Version).Should(Equal("v1"))
		//})
	})
})
