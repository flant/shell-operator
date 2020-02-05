package context

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
)

func parseContexts(contexts string) []BindingContext {
	parsedBindingContexts := []BindingContext{}
	json.Unmarshal([]byte(contexts), &parsedBindingContexts)
	return parsedBindingContexts
}

func Test_BindingContextGenerator(t *testing.T) {
	t.Run("Binding context generator test", func(t *testing.T) {
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
		assert.Equal(t, err, nil)

		// Synchronization contexts
		contexts, err := c.Run()
		assert.Equal(t, err, nil)

		parsedBindingContexts := parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].Type), "Synchronization")
		assert.Equal(t, len(parsedBindingContexts[0].Objects), 2)
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)

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
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Added")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 3)

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
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Modified")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 3)

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
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, string(parsedBindingContexts[0].WatchEvent), "Deleted")
		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)

		// Run schedule
		contexts, err = c.RunSchedule("* * * * *")
		assert.Equal(t, err, nil)
		parsedBindingContexts = parseContexts(contexts)

		assert.Equal(t, len(parsedBindingContexts[0].Snapshots["selected_pods"]), 2)
	})
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

func Test_PreferedGVR(t *testing.T) {
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

	g.Expect(len(bindingContexts[0].Snapshots["deployment"])).To(Equal(1))
	g.Expect(string(bindingContexts[0].Type)).To(Equal("Synchronization"))
}
