//go:build integration
// +build integration

package kube_event_manager_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/shell-operator/pkg/app"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
	. "github.com/flant/shell-operator/test/integration/suite"
	testutils "github.com/flant/shell-operator/test/utils"
)

func Test(t *testing.T) {
	RunIntegrationSuite(t, "KubeEventManager suite", "kube-event-manager-test")
}

var _ = Describe("Binding 'kubernetes' with kind 'Pod' should emit KubeEvent objects", func() {
	var KubeEventsManager kubeeventsmanager.KubeEventsManager

	BeforeEach(func() {
		fmt.Fprintf(GinkgoWriter, "Starting BeforeEach\n")
		KubeEventsManager = kubeeventsmanager.NewKubeEventsManager(context.Background(), KubeClient, log.NewNop())
		metricStorage := metric.NewStorageMock(GinkgoT())
		metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
			defer GinkgoRecover()
			fmt.Fprintf(GinkgoWriter, "HistogramObserve: %s, value: %f\n", metric, value)
			Expect(metric).To(Or(
				Equal(metrics.KubeJqFilterDurationSeconds),
				Equal(metrics.KubeEventDurationSeconds),
			))
			Expect(value).To(BeNumerically(">=", 0))
			Expect(labels).To(BeNil())
			Expect(buckets).To(BeNil())
		})
		metricStorage.GaugeSetMock.Set(func(metric string, value float64, labels map[string]string) {
			defer GinkgoRecover()
			fmt.Fprintf(GinkgoWriter, "GaugeSet: %s, value: %f\n", metric, value)
			Expect(metric).To(Equal(metrics.KubeSnapshotObjects))
			Expect(value).To(BeNumerically(">=", 0))
			Expect(labels).To(BeEmpty())
		})
		KubeEventsManager.WithMetricStorage(metricStorage)
		fmt.Fprintf(GinkgoWriter, "Finished BeforeEach\n")
	})

	AfterEach(func() {
		fmt.Fprintf(GinkgoWriter, "Starting AfterEach\n")
		KubeEventsManager.Stop()
		KubeEventsManager.Wait()
		fmt.Fprintf(GinkgoWriter, "Finished AfterEach\n")
	})

	Context("with configVersion: v1", func() {
		var monitorConfig *kubeeventsmanager.MonitorConfig

		BeforeEach(func() {
			fmt.Fprintf(GinkgoWriter, "Starting Context BeforeEach\n")
			monitorConfig = &kubeeventsmanager.MonitorConfig{
				Kind:                    "Pod",
				ApiVersion:              "v1",
				KeepFullObjectsInMemory: true,
				EventTypes: []kemtypes.WatchEventType{
					kemtypes.WatchEventAdded,
				},
				NamespaceSelector: &kemtypes.NamespaceSelector{
					NameSelector: &kemtypes.NameSelector{
						MatchNames: []string{"default"},
					},
				},
			}
			monitorConfig.Metadata.MonitorId = "test-abcd"

			err := KubeEventsManager.AddMonitor(monitorConfig)
			Expect(err).ShouldNot(HaveOccurred())
			KubeEventsManager.StartMonitor(monitorConfig.Metadata.MonitorId)
			fmt.Fprintf(GinkgoWriter, "Finished Context BeforeEach\n")
		})

		It("should have cached objects", func(ctx context.Context) {
			fmt.Fprintf(GinkgoWriter, "Starting test: should have cached objects\n")
			Expect(KubeEventsManager.HasMonitor(monitorConfig.Metadata.MonitorId)).Should(BeTrue())

			m := KubeEventsManager.GetMonitor(monitorConfig.Metadata.MonitorId)
			Expect(m).ShouldNot(BeNil())
			snapshot := m.Snapshot()
			Expect(snapshot).ShouldNot(BeNil())
			Expect(snapshot).Should(HaveLen(0), "No pods in default namespace. Snapshot at start should have no objects.")

			// Trigger metrics
			KubeEventsManager.MetricStorage().GaugeSet(metrics.KubeSnapshotObjects, 0, nil)
			KubeEventsManager.MetricStorage().HistogramObserve(metrics.KubeJqFilterDurationSeconds, 0, nil, nil)

			fmt.Fprintf(GinkgoWriter, "Finished test: should have cached objects\n")
		}, SpecTimeout(30*time.Second))

		When("Pod is Added", func() {
			JustBeforeEach(func() {
				fmt.Fprintf(GinkgoWriter, "Starting JustBeforeEach\n")
				app.SetupLogging(nil, log.NewNop())

				// Unlock KubeEvent emitting.
				m := KubeEventsManager.GetMonitor(monitorConfig.Metadata.MonitorId)
				m.EnableKubeEventCb()

				testutils.Kubectl(ContextName).Apply("default", "testdata/test-pod.yaml")
				fmt.Fprintf(GinkgoWriter, "Finished JustBeforeEach\n")
			})

			It("should return KubeEvent with type 'Event'", func(ctx context.Context) {
				fmt.Fprintf(GinkgoWriter, "Starting test: should return KubeEvent\n")
				ev := <-KubeEventsManager.Ch()

				fmt.Fprintf(GinkgoWriter, "Receive %#v\n", ev)

				Expect(ev.MonitorId).To(Equal(monitorConfig.Metadata.MonitorId))
				Expect(string(ev.Type)).To(Equal("Event"))

				Expect(ev.Objects).ShouldNot(BeNil())
				Expect(ev.Objects).Should(HaveLen(1))
				Expect(ev.Objects[0].Object.GetName()).Should(Equal("test"))
				Expect(ev.Objects[0].FilterResult).Should(BeNil(), "filterResult should be empty if monitor has no jqFilter or filterFunc")
				fmt.Fprintf(GinkgoWriter, "Finished test: should return KubeEvent\n")
			}, SpecTimeout(60*time.Second))

			AfterEach(func() {
				fmt.Fprintf(GinkgoWriter, "Starting AfterEach for Pod is Added\n")
				testutils.Kubectl(ContextName).Delete("default", "pod/test")
				fmt.Fprintf(GinkgoWriter, "Finished AfterEach for Pod is Added\n")
			})
		})
	})
})
