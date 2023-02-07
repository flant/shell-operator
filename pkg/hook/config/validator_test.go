package config

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/hashicorp/go-multierror"
)

func prepareConfigObj(g *WithT, input string) *VersionedUntyped {
	vu := NewDefaultVersionedUntyped()
	err := vu.Load([]byte(input))
	g.Expect(err).ShouldNot(HaveOccurred())
	return vu
}

func Test_Validate_V1_With_Error(t *testing.T) {
	g := NewWithT(t)

	var vu *VersionedUntyped
	var err error

	tests := []struct {
		name       string
		configText string
		fn         func()
	}{
		{
			"v1 config with error",
			`{
  "configVrsion":"v1",
  "schedule":{"name":"qwe"},
  "qwdqwd":"QWD"
}`,
			func() {
				g.Expect(err).Should(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
				g.Expect(err.(*multierror.Error).Error()).Should(And(
					MatchRegexp("configVrsion .*is a forbidden property"),
					MatchRegexp("qwdqwd .*is a forbidden property"),
					MatchRegexp("schedule .*must be of type array"),
				))
			},
		},
		{
			"v1 full config",
			`{
  "configVersion": "v1",
  "onStartup": 256,
  "schedule": [
    {
      "name": "qwe",
      "crontab": "*/5 * * * * *",
      "allowFailure": true,
      "includeSnapshotsFrom": ["monitor pods"],
      "queue": "offload"
    }
  ],
  "kubernetes": [
    {
      "name": "monitor pods",
      "executeHookOnEvent": ["Added", "Deleted", "Modified"],
      "executeHookOnSynchronization": false,
      "apiVersion": "v1",
      "kind": "Pod",
      "includeSnapshotsFrom": ["monitor pods"],
      "queue": "pods-offload",
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
}`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
			},
		},
		{
			"v0 full config",
			`{
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
        "update"
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
}`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vu = prepareConfigObj(g, tt.configText)
			s := GetSchema(vu.Version)
			err = ValidateConfig(vu.Obj, s, "root")
			// t.Logf("expected multierror was: %v", err)
			tt.fn()
		})
	}
}

func Test_Validate_V1_KubernetesValidating(t *testing.T) {
	g := NewWithT(t)

	var vu *VersionedUntyped
	var err error

	tests := []struct {
		name       string
		configText string
		fn         func()
	}{
		{
			"kubernetesValidating",
			`
configVersion: v1
kubernetesValidating:
- name: "myCrdValidator"
  group: main
  includeSnapshotsFrom:
  - myCrdBinding
  failurePolicy: Ignore
  labelSelector:
    matchExpressions:
      - key: someKey
        operator: Exists
  rules:
  - apiVersions: ["v1"]
    apiGroups: [""]
    resources: ["pods"]
    operations: ["*"]
  timeoutSeconds: 123
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vu = prepareConfigObj(g, tt.configText)
			t.Logf("version: %s", vu.Version)
			s := GetSchema(vu.Version)
			err = ValidateConfig(vu.Obj, s, "root")
			// t.Logf("expected multierror was: %v", err)
			tt.fn()
		})
	}
}
