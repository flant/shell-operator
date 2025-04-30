//go:build test
// +build test

// TODO: remove useless code

package utils

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type KubectlCmd struct {
	ContextName string
	ConfigPath  string
}

func Kubectl(context string) *KubectlCmd {
	return &KubectlCmd{
		ContextName: context,
	}
}

func (k *KubectlCmd) Apply(ns string, fileName string) {
	args := []string{"apply"}
	if ns != "" {
		args = append(args, "-n")
		args = append(args, ns)
	}
	args = append(args, "-f")
	args = append(args, fileName)

	cmd := exec.Command(GetKubectlPath(), args...)

	k.Succeed(cmd)
}

func (k *KubectlCmd) ReplaceForce(ns string, fileName string) {
	args := []string{"replace"}
	if ns != "" {
		args = append(args, "-n")
		args = append(args, ns)
	}
	args = append(args, "--force")
	args = append(args, "-f")
	args = append(args, fileName)

	cmd := exec.Command(GetKubectlPath(), args...)

	k.Succeed(cmd)
}

func (k *KubectlCmd) Delete(ns string, resourceName string) {
	args := []string{"delete"}
	if ns != "" {
		args = append(args, "-n")
		args = append(args, ns)
	}
	args = append(args, resourceName)

	cmd := exec.Command(GetKubectlPath(), args...)

	k.Succeed(cmd)
}

func (k *KubectlCmd) Succeed(cmd *exec.Cmd) {
	if k.ConfigPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", k.ConfigPath))
	}
	if k.ContextName != "" {
		cmd.Args = append(cmd.Args, "--context", k.ContextName)
	}
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())
	<-session.Exited
	Ω(session).Should(gexec.Exit())
	Ω(session.ExitCode()).Should(Equal(0))
}

func GetKubectlPath() string {
	path := os.Getenv("KUBECTL_BINARY_PATH")
	if path == "" {
		return "kubectl"
	}
	return path
}
