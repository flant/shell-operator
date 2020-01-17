package hook

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/gomega"
)

func Test_HookConfig_VersionedConfig_LoadAndValidate(t *testing.T) {
	g := NewWithT(t)
	var hookConfig *HookConfig
	var err error

	tests := []struct {
		name       string
		jsonConfig string
		testFn     func()
	}{
		{
			"load unknown version",
			`{"configVersion":"1.0.1-unknown","onStartup": 1}`,
			func() {
				g.Expect(err).Should(HaveOccurred())

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
				g.Expect(err).Should(HaveOccurred())
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
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.Version).To(Equal("v0"))
				g.Expect(hookConfig.V0).NotTo(BeNil())
				g.Expect(hookConfig.V1).To(BeNil())
				g.Expect(hookConfig.OnStartup).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup.Order).To(Equal(1.0))
			},
		},
		{
			"v0 with schedules and onKubernetesEvent",
			`{
              "onStartup": 1,
			  "schedule":[
			    {"name":"each 1 min", "crontab":"0 */1 * * * *"},
			    {"name":"each 5 min", "crontab":"0 */5 * * * *"}
			  ],
              "onKubernetesEvent":[
                {"name":"monitor pods", "kind":"pod", "allowFailure":true}
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v0"))
				g.Expect(hookConfig.V0).NotTo(BeNil())
				g.Expect(hookConfig.V1).To(BeNil())

				g.Expect(hookConfig.OnStartup).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup.Order).To(Equal(1.0))

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(1))
				pods := hookConfig.OnKubernetesEvents[0]
				g.Expect(pods.BindingName).To(Equal("monitor pods"))
				g.Expect(pods.Monitor.Kind).To(Equal("pod"))
				g.Expect(pods.AllowFailure).To(Equal(true))

				g.Expect(hookConfig.Schedules).Should(HaveLen(2))
				sch1min := hookConfig.Schedules[0]
				g.Expect(sch1min.BindingName).To(Equal("each 1 min"))
				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))

			},
		},

		{
			"v1 onStartup config",
			`{"configVersion":"v1","onStartup": 1}`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup.Order).To(Equal(1.0))
			},
		},
		{
			"v1 onStartup bad value",
			`{"configVersion":"v1","onStartup": e1}`,
			func() {
				g.Expect(err).Should(HaveOccurred())
				if merr, ok := err.(*multierror.Error); ok {
					fmt.Printf("expected error was: %s\n", multierror.ListFormatFunc(merr.Errors))
				} else {
					fmt.Printf("error was: %v\n", err)
				}
			},
		},
		{
			"v1 with schedules",
			`{
              "configVersion":"v1",
			  "schedule":[
			    {"name":"each 1 min", "crontab":"0 */1 * * * *"},
			    {"name":"each 5 min", "crontab":"0 */5 * * * *"}
			  ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(0))

				g.Expect(hookConfig.Schedules).Should(HaveLen(2))

				sch1min := hookConfig.Schedules[0]
				g.Expect(sch1min.BindingName).To(Equal("each 1 min"))

				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))

			},
		},
		{
			"v1 with onStartup and schedule",
			`{
              "configVersion":"v1",
              "onStartup": 212,
			  "schedule":[
			    {"name":"each 5 min", "crontab":"*/5 * * * *"},
			    {"name":"each 5 sec", "crontab":"*/5 * * * * *", "queue":"off-schedule"}
			  ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				g.Expect(hookConfig.OnStartup).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup.Order).To(Equal(212.0))

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(0))

				g.Expect(hookConfig.Schedules).Should(HaveLen(2))

				sch5min := hookConfig.Schedules[0]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))
				g.Expect(sch5min.Queue).To(Equal(""))

				sch5sec := hookConfig.Schedules[1]
				g.Expect(sch5sec.BindingName).To(Equal("each 5 sec"))
				g.Expect(sch5sec.Queue).To(Equal("off-schedule"))

			},
		},
		{
			"v1 with onStartup and kubernetes",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"name":"pods", "apiVersion":"v1", "kind":"Pod"},
			    {"name":"deployments", "apiVersion":"apps/v1", "kind":"Deployment", "includeSnapshotsFrom":["pods", "secrets"]},
			    {"name":"secrets", "apiVersion":"v1", "kind":"Secret", "queue":"offload"}
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.Schedules).Should(HaveLen(0))

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(3))

				pods := hookConfig.OnKubernetesEvents[0]
				g.Expect(pods.BindingName).To(Equal("pods"))

				deployments := hookConfig.OnKubernetesEvents[1]
				g.Expect(deployments.BindingName).To(Equal("deployments"))
				g.Expect(deployments.IncludeSnapshotsFrom).To(HaveLen(2))

				secrets := hookConfig.OnKubernetesEvents[2]
				g.Expect(secrets.BindingName).To(Equal("secrets"))
				g.Expect(secrets.Queue).To(Equal("offload"))
			},
		},
		{
			"v1 with snapshots in schedule",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"name":"pods", "apiVersion":"v1", "kind":"Pod"},
			    {"name":"secrets", "apiVersion":"v1", "kind":"Secret", "queue":"secrets"}
              ],
			  "schedule":[
			    {"name":"each 1 min", "crontab":"0 */1 * * * *", "includeKubernetesSnapshotsFrom":["pods", "secrets"]},
			    {"name":"each 5 min", "crontab":"0 */5 * * * *"},
			    {"name":"each 7 sec", "crontab":"*/7 * * * * *", "queue":"off-schedule"}
			  ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				g.Expect(hookConfig.OnStartup).To(BeNil())

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(2))

				g.Expect(hookConfig.Schedules).Should(HaveLen(3))

				sch1min := hookConfig.Schedules[0]
				g.Expect(sch1min.BindingName).To(Equal("each 1 min"))
				g.Expect(sch1min.Queue).To(Equal(""))
				g.Expect(sch1min.IncludeKubernetesSnapshotsFrom).Should(HaveLen(2))

				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))
				g.Expect(sch5min.Queue).To(Equal(""))

				sch7sec := hookConfig.Schedules[2]
				g.Expect(sch7sec.BindingName).To(Equal("each 7 sec"))
				g.Expect(sch7sec.Queue).To(Equal("off-schedule"))
			},
		},
		{
			"v1 yaml full config",
			`
configVersion: v1
onStartup: 112
kubernetes:
- name: pods
  apiVersion: v1
  kind: Pod
- name: secrets
  apiVersion: v1
  kind: Secret
  queue: secrets
schedule:
- name: each 1 min
  crontab: "0 */1 * * * *"
  includeKubernetesSnapshotsFrom: ["pods", "secrets"]
- name: each 5 min
  crontab: "0 */5 * * * *"
- name: each 7 sec
  crontab: "*/7 * * * * *"
  queue: off-schedule
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				g.Expect(hookConfig.OnStartup).NotTo(BeNil())
				g.Expect(hookConfig.OnStartup.Order).To(Equal(112.0))

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(2))

				g.Expect(hookConfig.Schedules).Should(HaveLen(3))

				sch1min := hookConfig.Schedules[0]
				g.Expect(sch1min.BindingName).To(Equal("each 1 min"))
				g.Expect(sch1min.Queue).To(Equal(""))
				g.Expect(sch1min.IncludeKubernetesSnapshotsFrom).Should(HaveLen(2))

				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))
				g.Expect(sch5min.Queue).To(Equal(""))

				sch7sec := hookConfig.Schedules[2]
				g.Expect(sch7sec.BindingName).To(Equal("each 7 sec"))
				g.Expect(sch7sec.Queue).To(Equal("off-schedule"))
			},
		},
		{
			"v1 kubernetes with empty nameSelector.matchNames",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {"name":"monitor pods", "apiVersion":"v1", "kind":"Pod",
                 "nameSelector":{"matchNames":[]}}
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(1))

				cfg := hookConfig.OnKubernetesEvents[0]

				g.Expect(cfg.Monitor).ToNot(BeNil())
				g.Expect(cfg.Monitor.NameSelector).ToNot(BeNil())
				g.Expect(cfg.Monitor.NameSelector.MatchNames).ToNot(BeNil())

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
				g.Expect(err).Should(HaveOccurred())
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
				g.Expect(err).Should(HaveOccurred())
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
				g.Expect(err).Should(HaveOccurred())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hookConfig = &HookConfig{}
			err = hookConfig.LoadAndValidate([]byte(test.jsonConfig))
			test.testFn()
		})
	}
}

// load kubernetes configs with errors
func Test_HookConfig_V1_Kubernetes_Validate(t *testing.T) {
	g := NewWithT(t)

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
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.NameSelector.MatchNames).Should(HaveLen(1))
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
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(2))
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
				g.Expect(err).Should(HaveOccurred())
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
				g.Expect(err).Should(HaveOccurred())
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
				g.Expect(err).Should(HaveOccurred())
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
