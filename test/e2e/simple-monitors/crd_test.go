// +build e2e

package simple_monitors_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/test/utils"
)

var _ = Describe("hook subscribed to CR objects", func() {
	It("should run hook after creating a cr object", func() {
		Kubectl(ConfigPath).Apply("", "testdata/crd/crd.yaml")

		stopCh := make(chan struct{})

		promScraper := NewPromScraper("http://localhost:9115/metrics")

		analyzer := NewJsonLogAnalyzer()
		analyzer.OnStop(func() {
			stopCh <- struct{}{}
		})

		startupLogMatcher := NewJsonLogMatcher(
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "Working dir")
			}),
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "Listen on")
			}),
			/*, func(r JsonLogRecord) error {
				//ExpectWithOffset(2, "qwe").Should(Equal("asd"))
				return fmt.Errorf("listen on fail")
			}*/
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "start Run")
			}),
			AwaitMatch(func(r JsonLogRecord) bool {
				// {"level":"info","msg":"Create new metric shell_operator_live_ticks","operator.component":"metricsStorage","time":"2019-11-12T22:34:18+03:00"}
				return r.FieldContains("operator.component", "metricsStorage") &&
					r.FieldContains("msg", "Create new metric shell_operator_live_ticks")
			}, func(_ JsonLogRecord) error {
				if err := promScraper.Scrape(); err != nil {
					return err
				}
				Ω(promScraper).Should(HaveMetric(PromMetric("shell_operator_live_ticks")))
				return nil
			}),
		)

		//stopLogMatcher := NewJsonLogMatcher(
		//	AwaitMatch(func(r JsonLogRecord) bool {
		//		return r.FieldContains("msg", "Working dir")
		//	}),
		//)

		errorMatcher := NewJsonLogMatcher(
			AwaitNotMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("level", "error")
			}),
		)

		hookMatcher := NewJsonLogMatcher(
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldEquals("msg", "Synchronization run") &&
					r.FieldEquals("output", "stdout")
			}),
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldEquals("msg", "Hook executed successfully") &&
					r.FieldEquals("hook", "crd-hook.sh")
			},
				func(_ JsonLogRecord) error {
					var err error

					//err = promScraper.Scrape()
					//Ω(err).ShouldNot(HaveOccurred())
					//Ω(promScraper).Should(HaveMetric(PromMetric("shell_operator_live_ticks")))
					//Ω(promScraper).Should(HaveMetricValue(PromMetric("shell_operator_hook_errors", "hook", "crd-hook.sh"), 0.0))

					_, err = kube.Kubernetes.CoreV1().Namespaces().Create(&v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crd-test",
						},
					})
					Ω(err).ShouldNot(HaveOccurred())
					Kubectl(ConfigPath).Apply("crd-test", "testdata/crd/cr-object.yaml")
					return nil
				}),
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "crontab is added") &&
					r.FieldEquals("output", "stdout")
			}),
		)

		analyzer.AddGroup(startupLogMatcher, errorMatcher) // .Timeout(300)
		analyzer.AddGroup(hookMatcher, errorMatcher)

		ShellOperatorStartWithAnalyzer(analyzer, "testdata/crd", stopCh)

		Expect(analyzer).To(FinishAllMatchersSuccessfully())
	})
})
