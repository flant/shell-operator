// +build e2e

package simple_monitors_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/test/utils"
)

type ShellOperatorTester struct {
	PromScraper *PromScraper
}

func (t *ShellOperatorTester) Operator() error {
	return nil
}

func (t *ShellOperatorTester) Tasks() *ShellOperatorTasks {
	return nil
}

func (t *ShellOperatorTester) ResetState() {
	return
}

type ShellOperatorTask struct {
}

func Task(taskType string, opts ...string) ShellOperatorTask {
	return ShellOperatorTask{}
}

type ShellOperatorTasks struct {
}

func (t *ShellOperatorTasks) Find(task ShellOperatorTask) error {
	return nil
}

func (t *ShellOperatorTasks) FindLastFinished(task ShellOperatorTask) error {
	return nil
}

func HaveSuccessfulInit() types.GomegaMatcher {
	return nil
}

func HaveError(task ShellOperatorTask) types.GomegaMatcher {
	return nil
}
func HaveStarted(task ShellOperatorTask) types.GomegaMatcher {
	return nil
}
func HaveFinished(task ShellOperatorTask) types.GomegaMatcher {
	return nil
}

func FinishBeforeRun(task ShellOperatorTask) types.GomegaMatcher {
	return nil
}

var LogRecords = []JsonLogRecord{}

func HandleLine(line string) {
	logRecord, err := NewJsonLogRecord().FromString(line)
	Ω(err).ShouldNot(HaveOccurred())

	LogRecords = append(LogRecords, logRecord)

	//// register finished hooks
	//if logRecord.HasField("hook") && logRecord.FieldHasPrefix("msg", "Hook executed successfully") {
	//	// success
	//	// hookRuns[hookName]
	//
	//	// hookRuns[time] = hookRun(hookname, msg, ...)
	//
	//}

}

func FindLastFinished(task map[string]string) {
	//	sql := `select * from tasks
	//where
	//  hook = ? and
	//  binding = ? and
	//  tasktype = ?
	//`
	//
	//	`select * from logs where
	//taskId = ?
	//`
	//
	//	`select * from logs
	//where
	//
	//msg =~ ''
	//
	//`
	for i := len(LogRecords) - 1; i >= 0; i-- {

	}
}

func ShellOperator(text string, config ShellOperatorOptions, body func(sh *ShellOperatorTester), timeout ...float64) bool {
	itBody := func() {
		// Start shell-operator process
		sht := &ShellOperatorTester{}

		stopCh := make(chan struct{})

		sht.PromScraper = NewPromScraper("http://localhost:9115/metrics")

		analyzer := NewJsonLogAnalyzer()
		analyzer.OnStop(func() {
			stopCh <- struct{}{}
		})

		operatorOptions := DefaultShellOperatorOptions(CurrentDir, ConfigPath).Merge(config)

		err := ExecShellOperator(operatorOptions, CommandOptions{
			StopCh:            stopCh,
			OutputLineHandler: analyzer.HandleLine,
		})
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

		body(sht)
	}

	return It(text, itBody, timeout...)
}

var _ = Describe("hook subscribed to CR objects", func() {
	ShellOperator("should run hook after creating a cr object",
		ShellOperatorOptions{
			WorkingDir: "testdata/crd",
		},
		func(sh *ShellOperatorTester) {
			Eventually(sh.Operator()).Should(HaveSuccessfulInit())

			Eventually(sh.Tasks()).Should(HaveError(Task("GlobalHookRun", "OnStartup", "hook1.sh")))

			sh.ResetState()

			t := sh.Tasks().FindLastFinished(Task("GlobalHookRun", "OnStartup", "hook1.sh"))

			//Eventually(sh.Tasks()).Should(HaveError(Task(GlobalHookRun, OnStartup, "hook1.sh")).AfterFinished(t.TaskId))

			t = sh.Tasks().FindLastFinished(Task("GlobalHookRun", "OnStartup", "hook1.sh"))

			t1 := sh.Tasks().Find(Task("GlobalHookRun", "OnStartup", "hook1.sh"))
			t2 := sh.Tasks().Find(Task("GlobalHookRun", "OnStartup", "hook2.sh"))

			Expect(t1).Should(FinishBeforeRun(t2))

			Eventually(sh.Tasks()).Should(HaveStarted(Task(ModuleDiscovery)))

			sh.ResetState()

			Eventually(sh.Tasks()).Should(HaveFinished(Task(GlobalHookRun, OnStartup, "hook1.sh")).AfterFinished(t.TaskId))

			Expect(sh.Errors()).ShouldNot(ShellOperatorErrorOccur())

			onStartupTask := sh.Tasks.Find(Task(GlobalHookRun, OnStartup, "hook1.sh"))
			Expect(onStartupTask).Should(StartBefore(Task(GlobalHookRun, Kubernetes, "Synchronization", "hook1.sh")))

			Expect(onStartupTask).Should(StartBefore(Task(GlobalHookRun, Kubernetes, "Synchronization", "hook1.sh")))

			Expect(sh.GlobalHookRuns.Get("hook1.sh").Stdout()).Expect(Contains("Pod"))

			Expect(sh.PromScraper).Should(HaveMetric(PromMetric("shell_operator_live_ticks")))

			Ω(sh).To(AwaitHookRun("hook1.sh").As(OnStartup))

			Expect(sh).To(StartHook("hook1.sh").As(OnStartup).Before("hook3.sh"))

			sh.ResetState()
		})

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
			AwaitMatch(func(r JsonLogRecord) bool {
				return r.FieldEquals("msg", "Hook executed successfully") &&
					r.FieldEquals("hook", "crd-hook.sh")
			}),
		)

		analyzer.AddGroup(startupLogMatcher, errorMatcher) // .Timeout(300)
		analyzer.AddGroup(hookMatcher, errorMatcher)

		ShellOperatorStartWithAnalyzer(analyzer, "testdata/crd", stopCh)

		Expect(analyzer).To(FinishAllMatchersSuccessfully())
	})
})
