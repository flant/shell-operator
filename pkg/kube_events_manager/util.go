package kube_events_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/flant/shell-operator/pkg/executor"
)

func resourceFilter(obj interface{}, jqFilter string) (res string, err error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	if jqFilter != "" {
		stdout, stderr, err := execJq(jqFilter, data)
		if err != nil {
			return "", fmt.Errorf("failed exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
		}

		res = stdout
	} else {
		res = string(data)
	}
	return
}

func execJq(jqFilter string, jsonData []byte) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/bin/jq", jqFilter)

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
