package config

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/api/admissionregistration/v1"
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
				g.Expect(sch5min.Queue).To(Equal("main"))

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
			    {"name":"each 1 min", "crontab":"0 */1 * * * *", "includeSnapshotsFrom":["pods", "secrets"]},
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
				g.Expect(sch1min.Queue).To(Equal("main"))
				g.Expect(sch1min.IncludeSnapshotsFrom).Should(HaveLen(2))

				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))
				g.Expect(sch5min.Queue).To(Equal("main"))

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
  includeSnapshotsFrom: ["pods", "secrets"]
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
				g.Expect(sch1min.Queue).To(Equal("main"))
				g.Expect(sch1min.IncludeSnapshotsFrom).Should(HaveLen(2))

				sch5min := hookConfig.Schedules[1]
				g.Expect(sch5min.BindingName).To(Equal("each 5 min"))
				g.Expect(sch5min.Queue).To(Equal("main"))

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
		{
			"v1 kubernetes error on empty value in matchLabels",
			`{
              "configVersion":"v1",
			  "kubernetes":[
			    {
                "apiVersion":"v1", "kind":"Pod",
                "nameSelector": {
                  "matchNames": ["app"]
                },
                "labelSelector":{"matchLabels":{"node-role.kubernetes.io/master":""}}
                }
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
			},
		},
		{
			"v1 includeSnapshotsFrom for group",
			`
              configVersion: v1
              schedule:
              - crontab: "* * * * *"
                group: pods
              kubernetes:
              - name: monitor_pods
                apiVersion: v1
                kind: Pod
                group: pods
              - name: monitor_configmaps
                apiVersion: v1
                kind: ConfigMap
                group: pods
            `,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				// Version
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())
				// Sections
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(2))
				g.Expect(hookConfig.Schedules).Should(HaveLen(1))

				// Schedule binding should has includeSnapshotsFrom
				s := hookConfig.Schedules[0]
				//g.Expect(s.BindingName).To(Equal("each 1 min"))
				g.Expect(s.Queue).To(Equal("main"))
				g.Expect(s.IncludeSnapshotsFrom).Should(HaveLen(2))

				kPods := hookConfig.OnKubernetesEvents[0]
				g.Expect(kPods.IncludeSnapshotsFrom).Should(HaveLen(2))
				kPods = hookConfig.OnKubernetesEvents[1]
				g.Expect(kPods.IncludeSnapshotsFrom).Should(HaveLen(2))
			},
		},
		{
			"v1 kubernetes error on group and includeSnapshotsFrom",
			`
              configVersion: v1
              schedule:
              - crontab: "* * * * *"
                group: pods
                includeSnapshotsFrom:
                - monitor_pods
			  kubernetes:
              - name: monitor_pods
                apiVersion: v1
                kind: Pod
                group: pods
              - name: monitor_configmaps
                apiVersion: v1
                kind: ConfigMap
                group: pods`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 executeHookOnSynchronization",
			`
              configVersion: v1
              kubernetes:
              - name: monitor_pods
                apiVersion: v1
                kind: Pod
                executeHookOnSynchronization: false
              - name: monitor_configmaps
                apiVersion: v1
                kind: ConfigMap
                executeHookOnSynchronization: true
              - name: monitor_secrets
                apiVersion: v1
                kind: Secret
            `,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				// Version
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				// Sections
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(3))

				pods := hookConfig.OnKubernetesEvents[0]
				g.Expect(pods.ExecuteHookOnSynchronization).Should(BeFalse())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].ExecuteHookOnSynchronization).Should(Equal("false"))

				cms := hookConfig.OnKubernetesEvents[1]
				g.Expect(cms.ExecuteHookOnSynchronization).Should(BeTrue())
				g.Expect(hookConfig.V1.OnKubernetesEvent[1].ExecuteHookOnSynchronization).Should(Equal("true"))

				secrets := hookConfig.OnKubernetesEvents[2]
				g.Expect(secrets.ExecuteHookOnSynchronization).Should(BeTrue())
				g.Expect(hookConfig.V1.OnKubernetesEvent[2].ExecuteHookOnSynchronization).Should(Equal(""))

				//g.Expect(kPods.IncludeSnapshotsFrom).Should(HaveLen(2))
				//kPods = hookConfig.OnKubernetesEvents[1]
				//g.Expect(kPods.IncludeSnapshotsFrom).Should(HaveLen(2))
			},
		},
		{
			"v1 bad executeHookOnSynchronization",
			`
              configVersion: v1
              kubernetes:
              - name: monitor_pods
                apiVersion: v1
                kind: Pod
                executeHookOnSynchronization: ok
            `,
			func() {
				g.Expect(err).Should(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring("executeHookOnSynchronization"))
			},
		},
		{
			"v1 kubernetesValidating",
			`
configVersion: v1
kubernetes:
- name: pods
  kind: pods
  group: main
kubernetesValidating:
- name: default.example.com
  rules:
  - apiVersions:
    - v1
    apiGroups:
    - crd-domain.io
    resources:
    - MyCustomResource
    operations:
    - "*"
- name: snapshots.example.com
  group: main
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1", "v1beta1"]
    resources: ["deployments", "replicasets"]
    scope: "Namespaced"  
- name: full.example.com
  includeSnapshotsFrom: ["pods"]
  rules:
  - apiVersions:
    - v1
    apiGroups:
    - crd-domain.io
    resources:
    - MyCustomResource
    operations:
    - "*"
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1", "v1beta1"]
    resources: ["deployments", "replicasets"]
    scope: "Namespaced"
  failurePolicy: Ignore
  sideEffects: NoneOnDryRun
  timeoutSeconds: 30
  namespace:
    labelSelector:
      matchLabels:
        foo: bar
  labelSelector:
    matchLabels:
      baz: bar
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				// Version
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				// Sections
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(1))
				g.Expect(hookConfig.KubernetesValidating).Should(HaveLen(3))

				// Section with default values
				cfg := hookConfig.KubernetesValidating[0]
				g.Expect(cfg.BindingName).To(Equal("default.example.com"))
				g.Expect(cfg.Webhook).ShouldNot(BeNil())
				wh := cfg.Webhook
				g.Expect(wh.NamespaceSelector).Should(BeNil())
				g.Expect(wh.ObjectSelector).Should(BeNil())
				g.Expect(wh.FailurePolicy).ShouldNot(BeNil())
				g.Expect(*wh.FailurePolicy).To(Equal(v1.Fail))
				g.Expect(wh.SideEffects).ShouldNot(BeNil())
				g.Expect(*wh.SideEffects).To(Equal(v1.SideEffectClassNone))
				g.Expect(wh.TimeoutSeconds).ShouldNot(BeNil())
				g.Expect(*wh.TimeoutSeconds).To(BeEquivalentTo(10))

				// Section with group should have updated IncludeSnapshotsFrom!
				cfg = hookConfig.KubernetesValidating[1]
				g.Expect(cfg.BindingName).To(Equal("snapshots.example.com"))
				g.Expect(cfg.Group).To(Equal("main"))
				g.Expect(cfg.IncludeSnapshotsFrom).To(BeEquivalentTo([]string{"pods"}))

				// Section with custom values
				cfg = hookConfig.KubernetesValidating[2]
				g.Expect(cfg.BindingName).To(Equal("full.example.com"))
				g.Expect(cfg.Webhook).ShouldNot(BeNil())
				wh = cfg.Webhook
				g.Expect(wh.NamespaceSelector).ShouldNot(BeNil())
				g.Expect(wh.ObjectSelector).ShouldNot(BeNil())
				g.Expect(wh.FailurePolicy).ShouldNot(BeNil())
				g.Expect(*wh.FailurePolicy).To(Equal(v1.Ignore))
				g.Expect(wh.SideEffects).ShouldNot(BeNil())
				g.Expect(*wh.SideEffects).To(Equal(v1.SideEffectClassNoneOnDryRun))
				g.Expect(wh.TimeoutSeconds).ShouldNot(BeNil())
				g.Expect(*wh.TimeoutSeconds).To(BeEquivalentTo(30))

			},
		},
		{
			"v1 kubernetesValidating name error",
			`
configVersion: v1
kubernetesValidating:
- name: snapshots
  rules:
  - operations: ["CREATE", "UPDATE"]
    apiGroups: ["apps"]
    apiVersions: ["v1", "v1beta1"]
    resources: ["deployments", "replicasets"]
`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 kubernetesValidating timeoutSeconds out of range",
			`
configVersion: v1
kubernetesValidating:
- name: snapshots.example.com
  rules:
  - operations: ["*"]
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
  timeoutSeconds: 32
`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 kubernetesCustomResourceConversion",
			`
configVersion: v1
kubernetes:
- name: pods
  kind: pods
  group: main
kubernetesCustomResourceConversion:
- name: v1alpha1-to-v1beta1
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: v1alpha1
    toVersion: v1beta1
- name: v1beta-to-v1
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: v1beta1
    toVersion: v1beta2
  - fromVersion: v1beta2
    toVersion: v1beta3
  - fromVersion: v1beta3
    toVersion: v1
  includeSnapshotsFrom: ["pods"]
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())

				// Version
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())

				// Sections
				g.Expect(hookConfig.OnStartup).To(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents).Should(HaveLen(1))
				g.Expect(hookConfig.KubernetesConversion).Should(HaveLen(2))

				// Conversion section with one conversion.
				cfg := hookConfig.KubernetesConversion[0]
				g.Expect(cfg.BindingName).To(Equal("v1alpha1-to-v1beta1"))
				g.Expect(cfg.Webhook).ShouldNot(BeNil())
				wh := cfg.Webhook
				g.Expect(wh.CrdName).Should(Equal("crontabs.stable.example.com"))
				g.Expect(wh.Rules).Should(HaveLen(1))

				// Conversion section with multiple conversions.
				cfg = hookConfig.KubernetesConversion[1]
				g.Expect(cfg.BindingName).To(Equal("v1beta-to-v1"))
				g.Expect(cfg.Webhook).ShouldNot(BeNil())
				wh = cfg.Webhook
				g.Expect(wh.CrdName).Should(Equal("crontabs.stable.example.com"))
				g.Expect(wh.Rules).Should(HaveLen(3))
			},
		},
		{
			"v1 kubernetesCustomResourceConversion with name only",
			`
configVersion: v1
kubernetesCustomResourceConversion:
- name: v1beta-to-v2
`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 kubernetesCustomResourceConversion with no crdName",
			`
configVersion: v1
kubernetesCustomResourceConversion:
- name: v1beta-to-v2
  conversions:
  - fromVersion: q
    toVersion: q
`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 kubernetesCustomResourceConversion with invalid conversion",
			`
configVersion: v1
kubernetesCustomResourceConversion:
- name: v1beta-to-v2
  crdName: crontabs.stable.example.com
  conversions:
  - fromVersion: q
    to: q
`,
			func() {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			"v1 settings",
			`
configVersion: v1
settings:
  executionMinInterval: 30s
  executionBurst: 1
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.Version).To(Equal("v1"))
				g.Expect(hookConfig.V0).To(BeNil())
				g.Expect(hookConfig.V1).NotTo(BeNil())
				g.Expect(hookConfig.Settings).NotTo(BeNil())
				g.Expect(hookConfig.Settings.ExecutionMinInterval).To(Equal(time.Duration(time.Second * 30)))
				g.Expect(hookConfig.Settings.ExecutionBurst).To(Equal(1))
			},
		},
		{
			"v1 settings with error",
			`
configVersion: v1
settings:
  executionMinInterval: 30ks
  executionBurst: 1
`,
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
			"watch event is empty array",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "watchEvent":[]
                }
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].WatchEventTypes).ShouldNot(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(0))
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).ShouldNot(BeNil())
			},
		},
		{
			"executeHookOnEvent is array",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "executeHookOnEvent":["Added", "Modified"]
                }
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].WatchEventTypes).Should(BeNil())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].ExecuteHookOnEvents).ShouldNot(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(2))
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).ShouldNot(BeNil())
			},
		},
		{
			"executeHookOnEvent is array in YAML",
			`
              configVersion: v1
              kubernetes:
              - name: OnCreateDeleteNamespace
                apiVersion: v1
                kind: Namespace
                executeHookOnEvent:
                - Added
                - Deleted
            `,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].WatchEventTypes).Should(BeNil())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].ExecuteHookOnEvents).ShouldNot(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(2))
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).ShouldNot(BeNil())
			},
		},
		{
			"executeHookOnEvent is empty array",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "executeHookOnEvent":[]
                }
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].WatchEventTypes).Should(BeNil())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].ExecuteHookOnEvents).ShouldNot(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(0))
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).ShouldNot(BeNil())
			},
		},
		{
			"no watch event or executeHookOnEvent",
			`{
              "configVersion":"v1",
              "kubernetes":[
                {
                  "apiVersion":"v1",
                  "kind":"Pod"
                }
              ]
            }`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].WatchEventTypes).Should(BeNil())
				g.Expect(hookConfig.V1.OnKubernetesEvent[0].ExecuteHookOnEvents).Should(BeNil())
				g.Expect(hookConfig.OnKubernetesEvents[0].Monitor.EventTypes).Should(HaveLen(3))
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
				g.Expect(err.Error()).To(MatchRegexp("kubernetes.watchEvent .*must be of type array: \"string\""))
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
				g.Expect(err.Error()).To(MatchRegexp("kubernetes.nameSelector.matchNames .*must be of type string: \"number\""))
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
                },{
                  "apiVersion":"v1",
                  "kind":"Pod",
                  "executeHookOnEvent": ["Added", "updated"]
                }
              ]
            }`,
			func() {
				g.Expect(err).Should(HaveOccurred())
				/*
								* kubernetes.nameSelector.matchNames must be of type string: "number"
					        	* kubernetes.labelSelector.matchLabels.foo must be of type string: "number"
					        	* kubernetes.labelSelector.matchExpressions.operand is a forbidden property
					        	* kubernetes.labelSelector.matchExpressions.operator is required
					        	* kubernetes.namespace.nameSelector.matchNames must be of type array: "object"
					        	* kubernetes.namespace.labelSelector.matchExpressions.operator should be one of [In NotIn Exists DoesNotExist]
					        	* kubernetes.fieldSelector.matchExpressions.operator should be one of [= == Equals != NotEquals]
					        	* kubernetes.fieldSelector.matchExpressions.value must be of type string: "number"

				*/
				//t.Logf("expected error was: %v\n", err)
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

func Test_MergeArrays(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name   string
		a1     []string
		a2     []string
		expect []string
	}{
		{
			"simple",
			[]string{"snap3", "snap2", "snap1"},
			[]string{"snap1", "snap4", "snap3"},
			[]string{"snap3", "snap2", "snap1", "snap4"},
		},
		{
			"empty",
			[]string{},
			[]string{},
			[]string{},
		},
		{
			"no_intersect",
			[]string{"snap3", "snap2", "snap1"},
			[]string{"snap11", "snap41", "snap31"},
			[]string{"snap3", "snap2", "snap1", "snap11", "snap41", "snap31"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := MergeArrays(test.a1, test.a2)
			g.Expect(res).To(Equal(test.expect))
		})
	}
}
