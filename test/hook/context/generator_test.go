package context

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
)

func parseContexts(contexts string) []BindingContext {
	var parsedBindingContexts []BindingContext
	_ = json.Unmarshal([]byte(contexts), &parsedBindingContexts)
	return parsedBindingContexts
}

func Test_BindingContextGenerator(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`
configVersion: v1
kubernetes:
- apiVersion: v1
  includeSnapshotsFrom:
  - selected_pods
  kind: Pod
  name: selected_pods
schedule:
- name: every_minute
  crontab: "* * * * *"
  includeSnapshotsFrom:
  - selected_pods
`, `
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Synchronization contexts
	contexts, err := c.Run()
	g.Expect(err).ShouldNot(HaveOccurred())

	parsedBindingContexts := parseContexts(contexts)

	g.Expect(string(parsedBindingContexts[0].Type)).To(Equal("Synchronization"))
	g.Expect(parsedBindingContexts[0].Objects).To(HaveLen(2))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))

	// Object added
	contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
---
apiVersion: v1
kind: Pod
metadata:
  name: pod3
spec:
  containers: []
`)
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts)

	g.Expect(string(parsedBindingContexts[0].WatchEvent)).To(Equal("Added"))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(3))

	// Object modified
	contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
---
apiVersion: v1
kind: Pod
metadata:
  name: pod3
spec:
  containers:
  - name: test
`)
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts)

	g.Expect(string(parsedBindingContexts[0].WatchEvent)).To(Equal("Modified"))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(3))

	// Object deleted
	contexts, err = c.ChangeState(`
---
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
`)
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts)

	g.Expect(string(parsedBindingContexts[0].WatchEvent)).To(Equal("Deleted"))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))

	// Run schedule
	contexts, err = c.RunSchedule("* * * * *")
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts)

	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))
}

func Test_RegisterCRD(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`configVersion: v1
kubernetes:
- apiVersion: my.crd.io/v1alpha1
  includeSnapshotsFrom:
  - selected_crds
  kind: MyResource
  name: selected_crds
`, "")
	g.Expect(err).ShouldNot(HaveOccurred())

	c.RegisterCRD("my.crd.io", "v1alpha1", "MyResource", true)

	bindingContexts, err := c.Run()

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(bindingContexts).To(ContainSubstring("Synchronization"))

	gvr, err := FakeCluster.FindGVR("my.crd.io/v1alpha1", "MyResource")
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(gvr).ShouldNot(BeNil())
	g.Expect(gvr.Group).To(Equal("my.crd.io"))
	g.Expect(gvr.GroupVersion().String()).To(Equal("my.crd.io/v1alpha1"))

	bindingContexts, err = c.ChangeState(`
apiVersion: my.crd.io/v1alpha1
kind: MyResource
metadata:
  name: my-res-obj
spec:
  data: foobar
---
apiVersion: my.crd.io/v1alpha1
kind: MyResource
metadata:
  name: my-res-obj-2
spec:
  data: baz
`)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(bindingContexts).To(ContainSubstring("MyResource"))

}

func Test_PreferredGVR(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`configVersion: v1
kubernetes:
- apiVersion: apps/v1
  includeSnapshotsFrom:
  - deployment
  kind: Deployment
  name: deployment
`, `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-res-obj-2
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	rawData, err := c.Run()
	g.Expect(err).ShouldNot(HaveOccurred())

	bindingContexts := parseContexts(rawData)

	g.Expect(bindingContexts[0].Snapshots["deployment"]).To(HaveLen(1))
	g.Expect(string(bindingContexts[0].Type)).To(Equal("Synchronization"))
}
