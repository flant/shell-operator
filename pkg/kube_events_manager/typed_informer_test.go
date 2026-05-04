package kubeeventsmanager

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// withTypedEnabled toggles the typed-informer flag for the duration of a test
// and restores the previous value at the end.
func withTypedEnabled(t *testing.T, v bool) {
	t.Helper()
	prev := IsTypedInformerEnabled()
	SetTypedInformerEnabled(v)
	t.Cleanup(func() { SetTypedInformerEnabled(prev) })
}

func Test_TypedInformer_DisabledByDefault(t *testing.T) {
	g := NewWithT(t)
	g.Expect(IsTypedInformerEnabled()).Should(BeFalse())
}

func Test_TypedInformer_RegistryHasCommonKinds(t *testing.T) {
	g := NewWithT(t)
	for _, gvr := range []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	} {
		g.Expect(IsTypedSupportedGVR(gvr)).Should(BeTrue(), "GVR %s should be registered", gvr.String())
	}
}

func Test_TypedInformer_RegistryDoesNotIncludeCRDs(t *testing.T) {
	g := NewWithT(t)
	g.Expect(IsTypedSupportedGVR(schema.GroupVersionResource{
		Group: "example.com", Version: "v1", Resource: "widgets",
	})).Should(BeFalse())
}

func Test_ToUnstructured_PassesThroughUnstructured(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{"kind": "X"}}
	g.Expect(toUnstructured(u)).Should(BeIdenticalTo(u))
}

func Test_ToUnstructured_NilInput(t *testing.T) {
	g := NewWithT(t)
	g.Expect(toUnstructured(nil)).Should(BeNil())
}

// Test_ToUnstructured_FromTypedPod confirms that a typed K8s API object pulled
// out of the typed informer cache (e.g. *corev1.Pod) can be converted back to
// *unstructured.Unstructured for downstream consumers (jq filters, hook
// binding contexts, snapshots).
func Test_ToUnstructured_FromTypedPod(t *testing.T) {
	g := NewWithT(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
			Namespace: "default",
			Labels:    map[string]string{"app": "demo"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "c", Image: "nginx:1.25"},
			},
		},
	}

	u := toUnstructured(pod)
	g.Expect(u).ShouldNot(BeNil())
	g.Expect(u.GetName()).Should(Equal("p1"))
	g.Expect(u.GetNamespace()).Should(Equal("default"))
	g.Expect(u.GetLabels()).Should(Equal(map[string]string{"app": "demo"}))

	// Spec is preserved through the conversion.
	containers, found, err := unstructured.NestedSlice(u.Object, "spec", "containers")
	g.Expect(err).Should(BeNil())
	g.Expect(found).Should(BeTrue())
	g.Expect(containers).Should(HaveLen(1))
}

// Test_TransformUnstructured_StripsManagedFields verifies the dynamic-cache
// dispatcher: for *unstructured.Unstructured payloads, ManagedFields and the
// kubectl last-applied annotation are stripped before the cache stores the
// object.
func Test_TransformUnstructured_StripsManagedFields(t *testing.T) {
	g := NewWithT(t)

	in := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":          "w",
				"managedFields": []any{map[string]any{"manager": "kubectl"}},
				"annotations": map[string]any{
					LastAppliedConfigAnnotation: "{\"big\":\"json\"}",
					"keep":                      "me",
				},
			},
		},
	}
	out, err := transformUnstructured(in)
	g.Expect(err).Should(BeNil())

	got, ok := out.(*unstructured.Unstructured)
	g.Expect(ok).Should(BeTrue())
	g.Expect(got.GetManagedFields()).Should(BeNil())
	g.Expect(got.GetAnnotations()).Should(Equal(map[string]string{"keep": "me"}))
}

// Test_TransformTyped_StripsManagedFields verifies the typed-cache dispatcher:
// for typed K8s API objects pulled off the wire by a typed informer,
// ManagedFields and the kubectl last-applied annotation are stripped via the
// metav1.Object accessors that every typed object implements.
func Test_TransformTyped_StripsManagedFields(t *testing.T) {
	g := NewWithT(t)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "p",
			Annotations: map[string]string{
				LastAppliedConfigAnnotation: "{\"big\":\"json\"}",
				"keep":                      "me",
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
			},
		},
	}

	out, err := transformTyped(pod)
	g.Expect(err).Should(BeNil())
	g.Expect(out).Should(BeIdenticalTo(pod))

	g.Expect(pod.ManagedFields).Should(BeNil())
	g.Expect(pod.Annotations).Should(Equal(map[string]string{"keep": "me"}))
}

func Test_MakeTransformFunc_DispatchesByCacheType(t *testing.T) {
	g := NewWithT(t)

	// typed=false: returns the unstructured stripper.
	dyn := makeTransformFunc(false)
	in := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"managedFields": []any{map[string]any{"manager": "kubectl"}},
			},
		},
	}
	out, err := dyn(in)
	g.Expect(err).Should(BeNil())
	g.Expect(out).Should(BeIdenticalTo(in))
	g.Expect(in.GetManagedFields()).Should(BeNil())

	// typed=true: returns the typed stripper.
	typed := makeTransformFunc(true)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
		},
	}
	outT, err := typed(pod)
	g.Expect(err).Should(BeNil())
	g.Expect(outT).Should(BeIdenticalTo(pod))
	g.Expect(pod.ManagedFields).Should(BeNil())
}

// Test_NewInformerForIndex_FallsBackForUnsupportedGVR confirms that an
// unrecognised GVR (e.g. a CRD) keeps using the dynamic informer, even when
// the typed code path is enabled.
func Test_NewInformerForIndex_FallsBackForUnsupportedGVR(t *testing.T) {
	g := NewWithT(t)
	withTypedEnabled(t, true)

	// We don't construct real clients here; newInformerForIndex only inspects
	// the registry to decide which factory to use. The actual factories will
	// be created with nil clients and never started, so they never make API
	// calls.
	g.Expect(IsTypedSupportedGVR(schema.GroupVersionResource{
		Group: "example.com", Version: "v1", Resource: "widgets",
	})).Should(BeFalse())
}
