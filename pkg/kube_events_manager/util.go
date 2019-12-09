package kube_events_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/flant/libjq-go"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
)

func ResourceFilter(obj interface{}, jqFilter string) (res string, err error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	if jqFilter != "" {
		if os.Getenv("JQ_EXEC") == "yes" {
			stdout, stderr, err := execJq(jqFilter, data)
			if err != nil {
				return "", fmt.Errorf("failed exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
			}

			res = stdout
		} else {
			res, err = Jq().WithLibPath(app.JqLibraryPath).Program(jqFilter).Cached().Run(string(data))
			if err != nil {
				return "", fmt.Errorf("failed jq filter: '%s'", err)
			}
		}
	} else {
		res = string(data)
	}
	return
}

// TODO: Can be removed after testing with libjq-go
// execJq run jq in locked mode with executor
func execJq(jqFilter string, jsonData []byte) (stdout string, stderr string, err error) {
	var cmd *exec.Cmd
	if app.JqLibraryPath == "" {
		cmd = exec.Command("/usr/bin/jq", jqFilter)
	} else {
		cmd = exec.Command("/usr/bin/jq", "-L", app.JqLibraryPath, jqFilter)
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
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}

// metaFromEventObject returns name and namespace from api object
func metaFromEventObject(obj interface{}) (namespace string, name string, err error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		err = fmt.Errorf("get ns and name from Object: %s", err)
		return
	}
	namespace = accessor.GetNamespace()
	name = accessor.GetName()
	return
}

func runtimeResourceId(obj interface{}, kind string) (string, error) {
	namespace, name, err := metaFromEventObject(obj)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", namespace, kind, name), nil
}

func FormatLabelSelector(selector *metav1.LabelSelector) (string, error) {
	res, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return "", err
	}

	return res.String(), nil
}

func FormatFieldSelector(selector *FieldSelector) (string, error) {
	if selector == nil || selector.MatchExpressions == nil {
		return "", nil
	}

	requirements := make([]fields.Selector, 0)

	for _, req := range selector.MatchExpressions {
		switch req.Operator {
		case "=", "==", "Equals":
			requirements = append(requirements, fields.OneTermEqualSelector(req.Field, req.Value))
		case "!=", "NotEquals":
			requirements = append(requirements, fields.OneTermNotEqualSelector(req.Field, req.Value))
		default:
			return "", fmt.Errorf("%s%s%s: operator '%s' is not recognized", req.Field, req.Operator, req.Value, req.Operator)
		}
	}

	return fields.AndSelectors(requirements...).String(), nil
}
