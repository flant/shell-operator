package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_VersionedUntyped_Load(t *testing.T) {
	var g = NewWithT(t)
	var vu *VersionedUntyped
	var err error

	var tests = []struct {
		name  string
		input string
		fn    func()
	}{
		{
			"load valid json object with version",
			`{"configVersion":"v1", "param1":"value1"}`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v1"))
				v, f, e := vu.GetString("param1")
				g.Expect(e).ToNot(HaveOccurred())
				g.Expect(f).To(BeTrue())
				g.Expect(v).To(Equal("value1"))
			},
		},
		{
			"load valid yaml with version",
			`
configVersion: v1
param1: value1
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v1"))
				v, f, e := vu.GetString("param1")
				g.Expect(e).ToNot(HaveOccurred())
				g.Expect(f).To(BeTrue())
				g.Expect(v).To(Equal("value1"))
			},
		},
		{
			"load valid json without version",
			`{"param1":"value1"}`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v0"))
				v, f, e := vu.GetString("param1")
				g.Expect(e).ToNot(HaveOccurred())
				g.Expect(f).To(BeTrue())
				g.Expect(v).To(Equal("value1"))
			},
		},
		{
			"load valid yaml without version",
			`
---
param1: value1
---
`,
			func() {
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v0"))
				v, f, e := vu.GetString("param1")
				g.Expect(e).ToNot(HaveOccurred())
				g.Expect(f).To(BeTrue())
				g.Expect(v).To(Equal("value1"))
			},
		},
		{
			"load shell-operator hook json configuration",
			`{
              "configVersion":"v1",
              "onStartup": 112,
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
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v1"))
				g.Expect(vu.Obj).To(HaveLen(4))
			},
		},
		{
			"load shell-operator hook yaml configuration",
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
				g.Expect(vu).ShouldNot(BeNil())
				g.Expect(vu.Version).To(Equal("v1"))
				g.Expect(vu.Obj).To(HaveLen(4))
			},
		},
		{
			"",
			``,
			func() {

			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vu = NewDefaultVersionedUntyped()
			err = vu.Load([]byte(tt.input))

			tt.fn()
		})
	}

}
