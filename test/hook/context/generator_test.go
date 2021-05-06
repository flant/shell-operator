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
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Synchronization contexts
	contexts, err := c.Run(`
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

	parsedBindingContexts := parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

	g.Expect(string(parsedBindingContexts[0].WatchEvent)).To(Equal("Deleted"))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))

	// Run schedule
	contexts, err = c.RunSchedule("* * * * *")
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts.Rendered)
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
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	c.RegisterCRD("my.crd.io", "v1alpha1", "MyResource", true)

	gvr, err := c.fakeCluster.FindGVR("my.crd.io/v1alpha1", "MyResource")
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(gvr).ShouldNot(BeNil())
	g.Expect(gvr.Group).To(Equal("my.crd.io"))
	g.Expect(gvr.GroupVersion().String()).To(Equal("my.crd.io/v1alpha1"))

	// Synchronization phase
	bindingContexts, err := c.Run("")

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(bindingContexts.Rendered).To(ContainSubstring("Synchronization"))

	// Event phase
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
	g.Expect(bindingContexts.Rendered).To(ContainSubstring("MyResource"))

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
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	contexts, err := c.Run(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-res-obj-2
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	bindingContexts := parseContexts(contexts.Rendered)

	g.Expect(bindingContexts[0].Snapshots["deployment"]).To(HaveLen(1))
	g.Expect(string(bindingContexts[0].Type)).To(Equal("Synchronization"))
}

func Test_Synchronization(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`
configVersion: v1
kubernetes:
- apiVersion: apps/v1
  includeSnapshotsFrom:
  - deployment
  kind: Deployment
  name: deployment
- apiVersion: v1
  kind: ConfigMap
  name: cms
  executeHookOnSynchronization: false
- apiVersion: v1
  includeSnapshotsFrom:
  - deployment
  - cms
  kind: Pod
  name: pods
- apiVersion: v1
  kind: Secret
  name: secrets-grouped
  group: group1
- apiVersion: v1
  kind: ConfigMap
  name: cms-grouped
  group: group1
- apiVersion: v1
  kind: Pod
  name: pods-grouped
  group: group1
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	contexts, err := c.Run(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-res-obj-2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-obj-1
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	bindingContexts := parseContexts(contexts.Rendered)

	g.Expect(bindingContexts).To(HaveLen(3))

	g.Expect(bindingContexts[0].Snapshots["deployment"]).To(HaveLen(1))
	g.Expect(string(bindingContexts[0].Type)).To(Equal("Synchronization"))

	g.Expect(string(bindingContexts[1].Type)).To(Equal("Synchronization"))
	g.Expect(bindingContexts[1].Snapshots["deployment"]).To(HaveLen(1))
	g.Expect(bindingContexts[1].Snapshots["cms"]).To(HaveLen(1))

	g.Expect(string(bindingContexts[2].Type)).To(Equal("Group"))
	g.Expect(bindingContexts[2].Snapshots["cms-grouped"]).To(HaveLen(1))
}

func Test_Groups(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`configVersion: v1
kubernetes:
- apiVersion: apps/v1
  group: main
  kind: Deployment
  name: deployment
- apiVersion: v1
  group: main
  kind: Secret
  name: secret
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	contexts, err := c.Run(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-res-obj-2
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret-obj-2
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	bindingContexts := parseContexts(contexts.Rendered)

	g.Expect(bindingContexts).To(HaveLen(1))
	g.Expect(string(bindingContexts[0].Type)).To(Equal("Group"))
	g.Expect(bindingContexts[0].Snapshots).To(HaveKey("secret"))
	g.Expect(bindingContexts[0].Snapshots).To(HaveKey("deployment"))
}

func Test_ExecuteOnSynchronization_false(t *testing.T) {
	g := NewWithT(t)

	c, err := NewBindingContextController(`
configVersion: v1
kubernetes:
- apiVersion: v1
  includeSnapshotsFrom:
  - selected_pods
  kind: Pod
  name: selected_pods
- apiVersion: v1
  includeSnapshotsFrom:
  - selected_pods
  kind: Pod
  name: selected_pods_nosync
  executeHookOnSynchronization: false
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Synchronization contexts
	contexts, err := c.Run(`
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

	parsedBindingContexts := parseContexts(contexts.Rendered)

	g.Expect(parsedBindingContexts).To(HaveLen(1))

	g.Expect(string(parsedBindingContexts[0].Type)).To(Equal("Synchronization"))
	g.Expect(parsedBindingContexts[0].Objects).To(HaveLen(2))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))

}

func Test_RunSchedule(t *testing.T) {
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
`)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Synchronization contexts
	contexts, err := c.Run(`
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

	parsedBindingContexts := parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

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
	parsedBindingContexts = parseContexts(contexts.Rendered)

	g.Expect(string(parsedBindingContexts[0].WatchEvent)).To(Equal("Deleted"))
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))

	// Run schedule
	contexts, err = c.RunSchedule("* * * * *")
	g.Expect(err).ShouldNot(HaveOccurred())
	parsedBindingContexts = parseContexts(contexts.Rendered)
	g.Expect(parsedBindingContexts[0].Snapshots["selected_pods"]).To(HaveLen(2))
}
