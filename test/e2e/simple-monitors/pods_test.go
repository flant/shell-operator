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

var _ = Describe("hook subscribed to Pods", func() {
	It("should run hook after creating a pod", func() {
		stopCh := make(chan struct{})

		assertionSteps := []struct {
			matcher   func(JsonLogRecord) bool
			onSuccess func()
		}{
			{
				matcher: func(r JsonLogRecord) bool {
					return r.FieldContains("msg", "Working dir")
				},
			},
			{
				matcher: func(r JsonLogRecord) bool {
					return r.FieldContains("msg", "Listen on")
				},
			},
			{
				matcher: func(r JsonLogRecord) bool {
					return r.FieldEquals("msg", "Synchronization run") &&
						r.FieldEquals("output", "stdout")
				},
			},
			{
				func(r JsonLogRecord) bool {
					return r.FieldEquals("msg", "Hook executed successfully") &&
						r.FieldEquals("hook", "pods-hook.sh")
				},
				func() {
					_, err := kube.Kubernetes.CoreV1().Namespaces().Create(&v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "monitor-pods-test",
						},
					})
					Ω(err).ShouldNot(HaveOccurred())
					Kubectl(ConfigPath).Apply("monitor-pods-test", "testdata/pods/test-pod.yaml")
				},
			},
			{
				func(r JsonLogRecord) bool {
					return r.FieldContains("msg", "Pod 'test' added") &&
						r.FieldEquals("output", "stdout")
				},
				func() {
					stopCh <- struct{}{}
				},
			},
		}
		index := 0

		err := ExecShellOperator(ShellOperatorOptions{
			CurrentDir: CurrentDir,
			Args:       []string{"start"},
			KubeConfig: ConfigPath,
			LogType:    "json",
			WorkingDir: filepath.Join(CurrentDir, "testdata", "pods"),
		}, CommandOptions{
			StopCh: stopCh,
			OutputLineHandler: func(line string) {
				fmt.Printf("Got line: %s\n", line)

				if index == len(assertionSteps) {
					return
				}

				Ω(line).Should(HavePrefix("{"))

				logRecord, err := NewJsonLogRecord().FromString(line)
				Ω(err).ShouldNot(HaveOccurred())

				step := assertionSteps[index]
				res := step.matcher(logRecord)
				if res {
					if step.onSuccess != nil {
						step.onSuccess()
					}
					index++
				}
			}})
		Ω(err).Should(HaveOccurred())
		Ω(err.Error()).Should(ContainSubstring("command is stopped"))
		Ω(index).Should(Equal(len(assertionSteps)))
	})
})
