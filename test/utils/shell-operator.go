// +build test

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/onsi/gomega/types"
)

type ShellOperatorOptions struct {
	CurrentDir string
	BinPath    string
	Command    string
	Args       []string
	LogType    string
	WorkingDir string
	KubeConfig string
}

func ExecShellOperator(operatorOpts ShellOperatorOptions, opts CommandOptions) error {
	if operatorOpts.BinPath == "" {
		operatorOpts.BinPath = GetShellOperatorPath()
	}
	cmd := exec.Command(operatorOpts.BinPath, operatorOpts.Args...)
	cmd.Env = os.Environ()
	if operatorOpts.CurrentDir != "" {
		cmd.Dir = operatorOpts.CurrentDir
	}
	// Environment variables
	if operatorOpts.LogType != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("LOG_TYPE=%s", operatorOpts.LogType))
	}
	if operatorOpts.KubeConfig != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SHELL_OPERATOR_KUBE_CONFIG=%s", operatorOpts.KubeConfig))
	}
	if operatorOpts.WorkingDir != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SHELL_OPERATOR_WORKING_DIR=%s", operatorOpts.WorkingDir))
	}
	return StreamedExecCommand(cmd, opts)
}

func GetShellOperatorPath() string {
	path := os.Getenv("SHELL_OPERATOR_BINARY_PATH")
	if path == "" {
		return "shell-operator"
	}
	return path
}

func BeShellOperatorStopped() types.GomegaMatcher {
	return &BeShellOperatorStoppedMatcher{}
}

type BeShellOperatorStoppedMatcher struct {
	err error
}

func (matcher *BeShellOperatorStoppedMatcher) Match(actual interface{}) (success bool, err error) {
	err, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("BeShellOperatorStopped must be passed an error. Got %T\n", actual)
	}

	matcher.err = err

	return matcher.err != nil && strings.Contains(matcher.err.Error(), "command is stopped"), nil
}

func (matcher *BeShellOperatorStoppedMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected ShellOperator stopped. Got error: %v", matcher.err)
}

func (matcher *BeShellOperatorStoppedMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected ShellOperator stopped with error")
}
