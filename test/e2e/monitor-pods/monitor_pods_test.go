// +build e2e

package monitor_pods_test

import (
	"fmt"
	"strings"

	//	"github.com/flant/shell-operator/pkg/kube"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		It("should start successful and output some settings", func() {
			expectedStrings := []struct {
				expected string
			}{
				{
					"Working dir",
				},
				{
					"Listen on",
				},
				{
					"Synchronization",
				},
			}
			expectedStringsIndex := 0

			configPath := utils.KindGetKubeconfigPath(clusterName)

			stopCh := make(chan struct{})

			err := utils.ExecShellOperator(utils.ShellOperatorOptions{
				Args:       []string{"start"},
				KubeConfig: configPath,
				LogType:    "json",
				WorkingDir: "testdata",
			}, utils.CommandOptions{
				StopCh: stopCh,
				OutputLineHandler: func(line string) {
					fmt.Printf("Got line: %s\n", line)
					//Ω(line).Should(ContainSubstring(expectedStrings[expectedStringsIndex].expected))
					if strings.Contains(line, expectedStrings[expectedStringsIndex].expected) {
						expectedStringsIndex++
					}
					if expectedStringsIndex == len(expectedStrings) {
						stopCh <- struct{}{}
					}
				}})
			Ω(err).Should(HaveOccurred())
			Ω(err.Error()).Should(ContainSubstring("command failed, exit code"))
			//switch e := err.(type) {
			//case *utils.CommandError:
			//	e.ExitCode
			//}
			Ω(expectedStringsIndex).Should(Equal(len(expectedStrings)))
		})
	})
})
