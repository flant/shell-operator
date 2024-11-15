package jq

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/flant/shell-operator/pkg/executor"
)

// jqExec is a subprocess implementation of the jq filtering.
func jqExec(jqFilter string, jsonData []byte, libPath string) (result string, err error) {
	var cmd *exec.Cmd
	if libPath == "" {
		cmd = exec.Command("jq", jqFilter)
	} else {
		cmd = exec.Command("jq", "-L", libPath, jqFilter)
	}

	var stdinBuf bytes.Buffer
	_, err = stdinBuf.WriteString(string(jsonData))
	if err != nil {
		panic(err)
	}
	cmd.Stdin = &stdinBuf
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = executor.Run(cmd)
	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())

	if err != nil {
		return "", fmt.Errorf("exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
	}

	return stdout, nil
}
