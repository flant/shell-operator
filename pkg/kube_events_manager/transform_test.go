package kubeeventsmanager

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_stripUnstructuredNoise_RemovesManagedFields(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name": "p",
				"managedFields": []any{
					map[string]any{
						"manager":   "kubectl",
						"operation": "Apply",
						"fieldsV1":  map[string]any{"f:spec": map[string]any{"f:replicas": map[string]any{}}},
					},
				},
			},
		},
	}

	stripUnstructuredNoise(obj)

	mf := obj.GetManagedFields()
	g.Expect(mf).Should(BeNil())
}

func Test_stripUnstructuredNoise_RemovesLastAppliedAnnotation(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "p",
				"annotations": map[string]any{
					LastAppliedConfigAnnotation: "{\"big\":\"json\"}",
					"keep":                      "me",
				},
			},
		},
	}

	stripUnstructuredNoise(obj)

	g.Expect(obj.GetAnnotations()).Should(Equal(map[string]string{"keep": "me"}))
}

func Test_stripUnstructuredNoise_RemovesAnnotationsMapWhenLastEntryStripped(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "p",
				"annotations": map[string]any{
					LastAppliedConfigAnnotation: "{\"big\":\"json\"}",
				},
			},
		},
	}

	stripUnstructuredNoise(obj)

	g.Expect(obj.GetAnnotations()).Should(BeNil())
}

func Test_stripUnstructuredNoise_NoopWhenNothingToStrip(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "p",
				"namespace": "default",
			},
		},
	}

	stripUnstructuredNoise(obj)

	g.Expect(obj.GetName()).Should(Equal("p"))
	g.Expect(obj.GetNamespace()).Should(Equal("default"))
	g.Expect(obj.GetAnnotations()).Should(BeNil())
	g.Expect(obj.GetManagedFields()).Should(BeNil())
}

func Test_transformUnstructured_NonUnstructuredPassThrough(t *testing.T) {
	g := NewWithT(t)

	type typed struct {
		metav1.ObjectMeta
		Spec string
	}
	in := &typed{Spec: "x"}

	out, err := transformUnstructured(in)
	g.Expect(err).Should(BeNil())
	g.Expect(out).Should(BeIdenticalTo(in))
}

func Test_transformUnstructured_StripsUnstructured(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"managedFields": []any{
					map[string]any{"manager": "kubectl"},
				},
			},
		},
	}

	out, err := transformUnstructured(obj)
	g.Expect(err).Should(BeNil())
	got, ok := out.(*unstructured.Unstructured)
	g.Expect(ok).Should(BeTrue())
	g.Expect(got.GetManagedFields()).Should(BeNil())
}
