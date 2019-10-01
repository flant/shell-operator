package hook

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func Test_HookConfig_VersionedConfig_LoadAndValidate(t *testing.T) {
	var hookConfig *HookConfig
	var err error

	tests := []struct {
		name     string
		jsonText string
		testFn   func()
	}{
		{
			"load unknown version",
			`{"configVersion":"1.0.1-unknown","onStartup": 1}`,
			func() {
				assert.Error(t, err)
				if merr, ok := err.(*multierror.Error); ok {
					fmt.Printf("expected error was: %s\n", multierror.ListFormatFunc(merr.Errors))
				} else {
					fmt.Printf("error was: %v\n", err)
				}
			},
		},
		{
			"load empty version",
			`{"configVersion":"","onStartup": 1}`,
			func() {
				assert.Error(t, err)
				if merr, ok := err.(*multierror.Error); ok {
					fmt.Printf("expected error was: %s\n", multierror.ListFormatFunc(merr.Errors))
				} else {
					fmt.Printf("error was: %v\n", err)
				}
			},
		},
		{
			"v0 onStartup config",
			`{"onStartup": 1}`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, "v0", hookConfig.Version)
					assert.NotNil(t, hookConfig.V0)
					assert.Nil(t, hookConfig.V1)
				}
			},
		},
		{
			"v1 onStartup config",
			`{"configVersion":"v1","onStartup": 1}`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, "v1", hookConfig.Version)
					assert.Nil(t, hookConfig.V0)
					assert.NotNil(t, hookConfig.V1)
				}
			},
		},
		{
			"v1 onStartup bad value",
			`{"configVersion":"v1","onStartup": e1}`,
			func() {
				assert.Error(t, err)
				if merr, ok := err.(*multierror.Error); ok {
					fmt.Printf("expected error was: %s\n", multierror.ListFormatFunc(merr.Errors))
				} else {
					fmt.Printf("error was: %v\n", err)
				}
			},
		},
		{
			"v1 with schedule",
			`{
              "configVersion":"v1",
			  "schedule":[
			    {"name":"each 1 min", "crontab":"0 */1 * * * *"},
			    {"name":"each 5 min", "crontab":"0 */5 * * * *"}
			  ]
            }`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, "v1", hookConfig.Version)
					assert.Nil(t, hookConfig.V0)
					assert.NotNil(t, hookConfig.V1)
					assert.Nil(t, hookConfig.OnStartup)
					assert.Len(t, hookConfig.Schedules, 2)
					assert.Len(t, hookConfig.OnKubernetesEvents, 0)
				}
			},
		},
		{
			"v1 with kubernetes",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"name":"monitor pods", "apiVersion":"v1", "kind":"Pod"},
			    {"name":"deployments", "apiVersion":"apps/v1", "kind":"Deployment"}
              ]
            }`,
			func() {
				if assert.NoError(t, err) {
					assert.Equal(t, "v1", hookConfig.Version)
					assert.Nil(t, hookConfig.V0)
					assert.NotNil(t, hookConfig.V1)
					assert.Nil(t, hookConfig.OnStartup)
					assert.Len(t, hookConfig.Schedules, 0)
					assert.Len(t, hookConfig.OnKubernetesEvents, 2)
				}
			},
		},
		{
			"v1 kubernetes error on missing kind",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"name":"monitor pods", "apiVersion":"v1"}
              ]
            }`,
			func() {
				assert.Error(t, err)
			},
		},
		{
			"v1 kubernetes error on bad apiVersion",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"apiVersion":"v1/12/wqe", "kind":"Pod"}
              ]
            }`,
			func() {
				assert.Error(t, err)
			},
		},
		{
			"v1 kubernetes error on metadata.name",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {
                "apiVersion":"v1", "kind":"Pod",
                "nameSelector": {
                  "matchNames": ["app"]
                },
                "fieldSelector": {
                  "matchExpressions": {
                    "field":"metadata.name",
                    "operator":"Equals",
                    "value": "app"
                  }
                }
              ]
            }`,
			func() {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hookConfig = &HookConfig{}
			err = hookConfig.LoadAndValidate([]byte(test.jsonText))
			test.testFn()
		})
	}
}

// load kubernetes configs with errors
func Test_HookConfig_V1_Kubernetes_Validate(t *testing.T) {
	var hookConfig *HookConfig
	var err error

	tests := []struct {
		name     string
		jsonText string
		testFn   func()
	}{
		{
			"name selector",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "nameSelector" : {
                    "matchNames" : [ "app" ]
                  }
                }
              ]
            }`,
			func() {
				if assert.NoError(t, err) {
					assert.Len(t, hookConfig.OnKubernetesEvents[0].Monitor.NameSelector.MatchNames, 1)
				}
			},
		},
		{
			"watch event is array",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "watchEvent":["Added", "Modified"]
                }
              ]
            }`,
			func() {
				if assert.NoError(t, err) {
					assert.Len(t, hookConfig.OnKubernetesEvents[0].Monitor.EventTypes, 2)
				}
			},
		},
		{
			"bad watch event",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "watchEvent": "Added"
                }
              ]
            }`,
			func() {
				assert.Error(t, err)
				t.Logf("expected error was: %v\n", err)
			},
		},
		{
			"nameSelector error",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "nameSelector": {
                    "matchNames": [20]
                  }
                }
              ]
            }`,
			func() {
				assert.Error(t, err)
				t.Logf("expected error was: %v\n", err)
			},
		},
		{
			"many errors at once",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "nameSelector": {
                    "matchNames": [20]
                  },
                  "labelSelector": {
                    "matchLabels": {
                      "foo": 28
                    },
                    "matchExpressions": [
                      {
                        "key": "foo",
                        "operand": "notin"
                      }
                    ]
                  }
                },{
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "namespace":{
                    "nameSelector": {
                      "matchNames": {"asd":"QWE"}
                    }
                  }
                },{
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "namespace":{
                    "labelSelector": {
                      "matchExpressions": [
                        {
                          "key": "foo",
                          "operator": "IsNotin"
                        }
                      ]
                    }
                  }
                },{
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "fieldSelector": {
                    "matchExpressions": [
                      {
                        "field": "foo",
                        "operator": "IsNotin",
                        "value": 2989
                      }
                    ]
                  }
                }
              ]
            }`,
			func() {
				assert.Error(t, err)
				t.Logf("expected error was: %v\n", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hookConfig = &HookConfig{}
			err = hookConfig.LoadAndValidate([]byte(test.jsonText))
			test.testFn()
		})
	}
}

//// Test loading legacy version of hook configuration
//func Test_HookConfig_V0(t *testing.T) {
//	configData := `
//{"schedule":[
//{"name":"each 1 min", "crontab":"0 */1 * * * *"},
//{"name":"each 5 min", "crontab":"0 */5 * * * *"}
//], "onKubernetesEvent":[
//{"name":"monitor pods", "kind":"pod", "allowFailure":true}
//]}
//`
//
//	hc := &HookConfig{}
//
//	err := json.Unmarshal([]byte(configData), hc)
//
//	if assert.NoError(t, err) {
//		assert.Equal(t, "v0", hc.Version)
//		assert.NotNil(t, hc.V0)
//		assert.Nil(t, hc.V1)
//	}
//
//	err = hc.Convert()
//	if assert.NoError(t, err) {
//		assert.Nil(t, hc.OnStartup)
//		assert.Len(t, hc.Schedules, 2)
//		assert.Len(t, hc.OnKubernetesEvents, 1)
//	}
//
//}
//
//// Test loading v1 version of hook configuration
//func Test_HookConfig_V1(t *testing.T) {
//	configData := `
//{"configVersion":"v1",
//"schedule":[
//{"name":"each 1 min", "crontab":"0 */1 * * * *"},
//{"name":"each 5 min", "crontab":"0 */5 * * * *"}
//], "kubernetes":[
//{"name":"monitor pods", "kind":"pod", "allowFailure":true}
//]}
//`
//
//	hc := &HookConfig{}
//
//	err := json.Unmarshal([]byte(configData), hc)
//	if assert.NoError(t, err) {
//		assert.Equal(t, "v1", hc.Version)
//		assert.Nil(t, hc.V0)
//		assert.NotNil(t, hc.V1)
//	}
//
//	err = hc.Convert()
//	if assert.NoError(t, err) {
//		assert.Nil(t, hc.OnStartup)
//		assert.Len(t, hc.Schedules, 2)
//		assert.Len(t, hc.OnKubernetesEvents, 1)
//	}
//}
//
//func Test_HookConfig_Convert_v1(t *testing.T) {
//
//	var hookConfig *HookConfig
//	var err error
//
//	tests := []struct {
//		name     string
//		jsonText string
//		testFn   func()
//	}{
//		{
//			"empty nameSelector.matchNames",
//			`{"configVersion":"v1", "kubernetes": [{"kind":"pod", "nameSelector":{}}]}`,
//			func() {
//				if assert.NoError(t, err) {
//					assert.Len(t, hookConfig.OnKubernetesEvents, 1)
//					assert.NotNil(t, hookConfig.OnKubernetesEvents[0].Monitor)
//					assert.NotNil(t, hookConfig.OnKubernetesEvents[0].Monitor.NameSelector)
//					// MatchNames array is nil
//					//assert.NotNil(t, hookConfig.OnKubernetesEvents[0].Monitor.NameSelector.MatchNames)
//					assert.Len(t, hookConfig.OnKubernetesEvents[0].Monitor.NameSelector.MatchNames, 0)
//				}
//			},
//		},
//	}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			hookConfig = &HookConfig{}
//			err = nil
//			err = json.Unmarshal([]byte(test.jsonText), hookConfig)
//			if assert.NoError(t, err) {
//				err = hookConfig.Convert()
//				test.testFn()
//			}
//		})
//	}
//}
