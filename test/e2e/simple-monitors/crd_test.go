// +build e2e

package simple_monitors_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/test/utils"
)

var _ = Describe("hook subscribed to CR objects", func() {
	It("should run hook after creating a cr object", func() {
		//Kubectl(ConfigPath).Apply("", "testdata/crd/crd.yaml")

		stopCh := make(chan struct{})

		//RegisterFailHandler(func(message string, callerSkip ...int) {
		//fmt.Printf("send stop to StopCh\n")
		//stopCh <- struct{}{}
		//fmt.Printf("send stop to StopCh Done\n")
		//	Fail(message, callerSkip...)
		//})

		analyzer := NewJsonLogAnalyzer()
		analyzer.OnStop(func() {

			fmt.Printf("send stop to StopCh\n")
			stopCh <- struct{}{}
			fmt.Printf("send stop to StopCh Done\n")
		})

		startupLogMatcher := NewJsonLogMatcher(
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "Working dir")
			}),
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "Listen on")
			}),
			/*, func(r JsonLogRecord) error {
				//ExpectWithOffset(2, "qwe").Should(Equal("asd"))
				return fmt.Errorf("listen on fail")
			}*/
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "start Run")
			}),
		)

		errorMatcher := NewJsonLogMatcher(
			ShouldNotMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("level", "error")
			}),
		)

		_ = NewJsonLogMatcher(
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldEquals("msg", "Synchronization run") &&
					r.FieldEquals("output", "stdout")
			}),
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldEquals("msg", "Hook executed successfully") &&
					r.FieldEquals("hook", "crd-hook.sh")
			},
				func(_ JsonLogRecord) error {
					// metrics := MetricsScrape(":9115")
					// Metrics.HasMetric("shell_operator_live_ticks")
					// metrics.
					_, err := kube.Kubernetes.CoreV1().Namespaces().Create(&v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "crd-test",
						},
					})
					Ω(err).ShouldNot(HaveOccurred())
					Kubectl(ConfigPath).Apply("crd-test", "testdata/crd/cr-object.yaml")
					return nil
				}),
			ShouldMatch(func(r JsonLogRecord) bool {
				return r.FieldContains("msg", "crontab is added") &&
					r.FieldEquals("output", "stdout")
			},
				func(_ JsonLogRecord) error {
					stopCh <- struct{}{}
					return nil
				}),
		)

		analyzer.Add(startupLogMatcher, errorMatcher) // .Timeout(300)
		//errorMatcher.Reset()
		//analyzer.Add(hookMatcher, errorMatcher)

		err := ExecShellOperator(ShellOperatorOptions{
			CurrentDir: CurrentDir,
			Args:       []string{"start"},
			KubeConfig: ConfigPath,
			LogType:    "json",
			WorkingDir: filepath.Join(CurrentDir, "testdata", "crd"),
		}, CommandOptions{
			StopCh:            stopCh,
			OutputLineHandler: analyzer.HandleLine,
		})

		Ω(err).Should(HaveOccurred())
		Ω(err.Error()).Should(ContainSubstring("command is stopped"))

		Ω(analyzer.Finished()).Should(BeTrue())
		Ω(analyzer.Error()).ShouldNot(HaveOccurred())
	})
})
