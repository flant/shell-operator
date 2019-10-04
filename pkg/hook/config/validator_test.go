package config

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func Test_Validate_V1_With_Error(t *testing.T) {
	data := `{
  "configVrsion":"v1",
  "schedule":{"name":"qwe"},
  "qwdqwd":"QWD"
}`

	dataObj := make(map[string]interface{})
	e := json.Unmarshal([]byte(data), &dataObj)
	fmt.Printf("dataObj: %+v\nerr: %v\n", dataObj, e)

	s := GetSchema("v1")

	err := ValidateConfig(dataObj, s, "root")

	if assert.Error(t, err) {
		assert.IsType(t, &multierror.Error{}, err)
		t.Logf("expected multierror was: %v", err)
	}
}

func Test_Validate_V1_Full(t *testing.T) {
	data := `{
  "configVersion": "v1",
  "onStartup": 256,
  "schedule": [
    {
      "name": "qwe",
      "crontab": "*/5 * * * * *",
      "allowFailure": true
    }
  ],
  "kubernetes": [
    {
      "name": "monitor pods",
      "watchEvent": ["Added", "Deleted", "Modified"],
      "apiVersion": "v1",
      "kind": "Pod",
      "labelSelector": {
        "matchLabels": {
          "app": "app",
          "heritage": "test"
        }
      },
      "fieldSelector": {
        "matchExpressions": [{
          "field": "metadata.name",
          "operator": "==",
          "value": "pod-one-two"
        }]
      },
      "namespace": {
        "nameSelector": {
          "matchNames": [
            "default"
          ]
        },
        "labelSelector": {
          "matchExpressions": [{
            "key": "app",
            "operator": "In",
            "values": ["one", "two"]
          }]
        }
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true,
      "resynchronizationPeriod": "10s"
    }
  ]
}`

	dataObj := make(map[string]interface{})
	err := json.Unmarshal([]byte(data), &dataObj)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	s := GetSchema("v1")
	err = ValidateConfig(dataObj, s, "")
	assert.NoError(t, err)
}

func Test_Validate_V0_Full(t *testing.T) {
	data := `{
  "onStartup": 256,
  "schedule": [
    {
      "name": "qwe",
      "crontab": "*/5 * * * * *",
      "allowFailure": true
    }
  ],
  "onKubernetesEvent": [
    {
      "name": "monitor pods",
      "kind": "Pod",
      "event": [
        "add",
        "uupdate"
      ],
      "selector": {
        "matchLabels": {
          "app": "app",
          "heritage": "test"
        }
      },
      "objectName": "pod-one-two",
      "namespaceSelector": {
        "matchNames": [
          "default"
        ]
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true
    }
  ]
}`

	dataObj := make(map[string]interface{})
	err := json.Unmarshal([]byte(data), &dataObj)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	s := GetSchema("v0")
	err = ValidateConfig(dataObj, s, "")
	assert.NoError(t, err)
}
