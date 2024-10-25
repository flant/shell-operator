package executor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type CmdUsage struct {
	Sys    time.Duration
	User   time.Duration
	MaxRss int64
}

func Run(cmd *exec.Cmd) error {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	return cmd.Run()
}

func RunAndLogLines(cmd *exec.Cmd, logLabels map[string]string) (*CmdUsage, error) {
	// TODO observability
	stdErr := bytes.NewBuffer(nil)
	logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))
	stdoutLogEntry := logEntry.WithField("output", "stdout")
	stderrLogEntry := logEntry.WithField("output", "stderr")

	logEntry.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)

	plo := &proxyJSONLogger{stdoutLogEntry, make([]byte, 0), app.LogProxyHookJSON}
	ple := &proxyJSONLogger{stderrLogEntry, make([]byte, 0), app.LogProxyHookJSON}
	cmd.Stdout = plo
	cmd.Stderr = io.MultiWriter(ple, stdErr)

	err := cmd.Run()
	if err != nil {
		if len(stdErr.Bytes()) > 0 {
			return nil, fmt.Errorf("%s", stdErr.String())
		}
		return nil, err
	}

	var usage *CmdUsage
	if cmd.ProcessState != nil {
		usage = &CmdUsage{
			Sys:  cmd.ProcessState.SystemTime(),
			User: cmd.ProcessState.UserTime(),
		}
		// FIXME Maxrss is Unix specific.
		sysUsage := cmd.ProcessState.SysUsage()
		if v, ok := sysUsage.(*syscall.Rusage); ok {
			// v.Maxrss is int32 on arm/v7
			usage.MaxRss = int64(v.Maxrss) //nolint:unconvert
		}
	}

	return usage, err
}

type proxyJSONLogger struct {
	*log.Entry

	buf []byte

	logProxyHookJSON bool
}

func (pj *proxyJSONLogger) Write(p []byte) (int, error) {
	if !pj.logProxyHookJSON {
		pj.writerScanner(p)

		return len(p), nil
	}

	pj.buf = append(pj.buf, p...)

	var line interface{}
	err := json.Unmarshal(pj.buf, &line)
	if err != nil {
		if err.Error() == "unexpected end of JSON input" {
			return len(p), nil
		}
		return len(p), err
	}

	logMap, ok := line.(map[string]interface{})
	if !ok {
		pj.Debugf("json log line not map[string]interface{}: %v", line)
		// fall back to using the logger
		pj.Info(string(p))
		return len(p), err
	}

	for k, v := range pj.Data {
		logMap[k] = v
	}

	logLine, _ := json.Marshal(logMap)

	logEntry := pj.WithField(app.ProxyJsonLogKey, true)

	logEntry.Log(log.FatalLevel, string(logLine))

	return len(p), nil
}

func (pj *proxyJSONLogger) writerScanner(p []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(p))

	// Set the buffer size to the maximum token size to avoid buffer overflows
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)

	// Define a split function to split the input into chunks of up to 64KB
	chunkSize := bufio.MaxScanTokenSize // 64KB
	splitFunc := func(data []byte, atEOF bool) (int, []byte, error) {
		if len(data) >= chunkSize {
			return chunkSize, data[:chunkSize], nil
		}

		return bufio.ScanLines(data, atEOF)
	}

	// Use the custom split function to split the input
	scanner.Split(splitFunc)

	// Scan the input and write it to the logger using the specified print function
	for scanner.Scan() {
		// prevent empty logging
		str := strings.TrimSpace(scanner.Text())
		if str == "" {
			continue
		}

		pj.Entry.Info(str)
	}

	// If there was an error while scanning the input, log an error
	if err := scanner.Err(); err != nil {
		pj.Entry.Errorf("Error while reading from Writer: %s", err)
	}
}

func Output(cmd *exec.Cmd) (output []byte, err error) {
	// TODO context: hook name, hook phase, hook binding
	// TODO observability
	log.Debugf("Executing command '%s' in '%s' dir", strings.Join(cmd.Args, " "), cmd.Dir)
	output, err = cmd.Output()
	return
}

func MakeCommand(dir string, entrypoint string, args []string, envs []string) *exec.Cmd {
	cmd := exec.Command(entrypoint, args...)
	cmd.Env = append(cmd.Env, envs...)
	cmd.Dir = dir
	return cmd
}
