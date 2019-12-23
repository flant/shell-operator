package kube_events_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/flant/libjq-go"
	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
)

// ApplyJqFilter filter object json representation with jq expression, calculate checksum
// over result and return ObjectAndFilterResult. If jqFilter is empty, no filter
// is required and checksum is calculated over full json representation of the object.
func ApplyJqFilter(jqFilter string, obj *unstructured.Unstructured) (*ObjectAndFilterResult, error) {
	res := &ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilter
	res.Metadata.ResourceId = ResourceId(obj)

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	if jqFilter == "" {
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(string(data))
	} else {
		var err error
		var filtered string
		if os.Getenv("JQ_EXEC") == "yes" {
			stdout, stderr, err := execJq(jqFilter, data)
			if err != nil {
				return nil, fmt.Errorf("failed exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
			}

			filtered = stdout
		} else {
			filtered, err = Jq().WithLibPath(app.JqLibraryPath).Program(jqFilter).Cached().Run(string(data))
			if err != nil {
				return nil, fmt.Errorf("failed jq filter: '%s'", err)
			}
		}
		if filtered == "" && len(data) > 0 {
			log.Errorf("Possible Bug!!! jq '%s' return an unusual empty result, but input was not empty", jqFilter)
		}
		res.FilterResult = filtered
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(filtered)
	}
	return res, nil
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

func ResourceId(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())
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
