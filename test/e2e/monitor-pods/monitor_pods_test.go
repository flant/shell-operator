// +build e2e

package monitor_pods_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/test/utils"
)

var _ = Describe("Shell-operator kubernetes hook", func() {
	var clusterName = "monitor-pods-test"

	SynchronizedBeforeSuite(func() (res []byte) {
		Ω(utils.KindCreateCluster(clusterName)).Should(Succeed())
		return
	}, func([]byte) {
		// Initialize kube client out-of-cluster
		configPath := utils.KindGetKubeconfigPath(clusterName)
		Ω(kube.Init(kube.InitOptions{KubeContext: "", KubeConfig: configPath})).Should(Succeed())
	})

	SynchronizedAfterSuite(func() {}, func() {
		Ω(utils.KindDeleteCluster(clusterName)).Should(Succeed())
	})

	Context("", func() {
		It("should run hook after creating a pod", func() {
			currentDir, _ := os.Getwd()

			configPath := utils.KindGetKubeconfigPath(clusterName)
			stopCh := make(chan struct{})

			assertions := []struct {
				matchFn   func(map[string]string) bool
				successFn func()
			}{
				{
					func(l map[string]string) bool {
						return utils.FieldContains(l, "msg", "Working dir")
					},
					func() {},
				},
				{
					func(l map[string]string) bool {
						return utils.FieldContains(l, "msg", "Listen on")
					},
					func() {},
				},
				{
					func(l map[string]string) bool {
						return utils.FieldEquals(l, "msg", "Synchronization run") &&
							utils.FieldEquals(l, "output", "stdout")
					},
					func() {},
				},
				{
					func(l map[string]string) bool {
						return utils.FieldEquals(l, "msg", "Hook executed successfully") &&
							utils.FieldEquals(l, "hook", "pods-hook.sh")
					},
					func() {
						_, err := kube.Kubernetes.CoreV1().Namespaces().Create(&v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: "monitor-pods-test",
							},
						})
						Ω(err).ShouldNot(HaveOccurred())
						//utils.KubectlCreateNamespace("monitor-pods-test")
						utils.Kubectl(configPath).Apply("monitor-pods-test", "testdata/test-pod.yaml")
					},
				},
				{
					func(l map[string]string) bool {
						return utils.FieldContains(l, "msg", "Pod 'test' added") &&
							utils.FieldEquals(l, "output", "stdout")
					},
					func() {
						stopCh <- struct{}{}
					},
				},
			}
			index := 0

			err := utils.ExecShellOperator(utils.ShellOperatorOptions{
				CurrentDir: currentDir,
				Args:       []string{"start"},
				KubeConfig: configPath,
				LogType:    "json",
				WorkingDir: filepath.Join(currentDir, "testdata"),
			}, utils.CommandOptions{
				StopCh: stopCh,
				OutputLineHandler: func(line string) {
					fmt.Printf("Got line: %s\n", line)

					if index == len(assertions) {
						return
					}

					Ω(line).Should(HavePrefix("{"))
					var logLine map[string]string
					err := json.Unmarshal([]byte(line), &logLine)
					Ω(err).ShouldNot(HaveOccurred())

					res := assertions[index].matchFn(logLine)
					if res {
						assertions[index].successFn()
						index++
					}
				}})
			Ω(err).Should(HaveOccurred())
			Ω(err.Error()).Should(ContainSubstring("command is stopped"))
			Ω(index).Should(Equal(len(assertions)))
		})
	})
})
