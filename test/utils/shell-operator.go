// +build test

package utils

import (
	"fmt"
	"os"
	"os/exec"
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
