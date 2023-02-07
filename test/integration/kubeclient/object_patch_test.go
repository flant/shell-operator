//go:build integration
// +build integration

package kubeclient_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "gopkg.in/satori/go.uuid.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/flant/shell-operator/pkg/kube/object_patch"
	. "github.com/flant/shell-operator/test/integration/suite"
)

const (
	creationNamespace = "creation"
	deletionNamespace = "deletion"
	patchNamespace    = "patch"
)

var testObject = &corev1.ConfigMap{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test",
	},
	Data: map[string]string{
		"firstField":  "test1",
		"secondField": "test2",
	},
}

var _ = Describe("Kubernetes API object patching", func() {
	Context("creating an object", func() {
		var (
			testCM         *corev1.ConfigMap
			unstructuredCM *unstructured.Unstructured
		)

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = creationNamespace + "-" + randomSuffix()

			var err error
			unstructuredCM, err = generateUnstructured(testCM)
			Expect(err).To(Succeed())

			Expect(ensureNamespace(testCM.Namespace)).To(Succeed())
			Expect(ensureTestObject(testCM.Namespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(testCM.Namespace))
		})

		It("should fail to Create() an object if it already exists", func() {
			err := ObjectPatcher.ExecuteOperation(object_patch.NewCreateOperation(unstructuredCM))
			Expect(err).To(Not(Succeed()))
		})

		It("should should successfully CreateOrUpdate() an object even if it already exists", func() {
			newTestCM := testCM.DeepCopy()
			newTestCM.Data = map[string]string{
				"firstField":  "test3",
				"secondField": "test4",
			}

			unstructuredNewTestCM, err := generateUnstructured(newTestCM)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(object_patch.NewCreateOperation(unstructuredNewTestCM, object_patch.UpdateIfExists()))
			Expect(err).To(Succeed())

			cm, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), newTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(cm.Data).To(Equal(newTestCM.Data))
		})

		It("should successfully CreateOrUpdate() an object if id does not yet exist", func() {
			separateTestCM := testCM.DeepCopy()
			separateTestCM.Name = "separate-test"

			unstructuredSeparateTestCM, err := generateUnstructured(separateTestCM)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(object_patch.NewCreateOperation(unstructuredSeparateTestCM, object_patch.UpdateIfExists()))
			Expect(err).To(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), separateTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
		})
	})

	Context("deleting an object", func() {
		var testCM *corev1.ConfigMap

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = deletionNamespace + "-" + randomSuffix()

			Expect(ensureNamespace(testCM.Namespace)).To(Succeed())
			Expect(ensureTestObject(testCM.Namespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(testCM.Namespace))
		})

		It("should successfully delete an object", func() {
			err := ObjectPatcher.ExecuteOperation(object_patch.NewDeleteOperation(
				testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name,
				object_patch.InBackground()))
			Expect(err).Should(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), testCM.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should successfully delete an object if it doesn't exist", func() {
			err := ObjectPatcher.ExecuteOperation(object_patch.NewDeleteOperation(testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).Should(Succeed())
		})
	})

	Context("patching an object", func() {
		var testCM *corev1.ConfigMap

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = patchNamespace + "-" + randomSuffix()

			Expect(ensureNamespace(testCM.Namespace)).To(Succeed())
			Expect(ensureTestObject(testCM.Namespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(testCM.Namespace))
		})

		It("should successfully JQPatch an object", func() {
			err := ObjectPatcher.ExecuteOperation(object_patch.NewFromOperationSpec(object_patch.OperationSpec{
				Operation:  object_patch.JQPatch,
				ApiVersion: testCM.APIVersion,
				Kind:       testCM.Kind,
				Namespace:  testCM.Namespace,
				Name:       testCM.Name,
				JQFilter:   `.data.firstField = "JQPatched"`,
			}))
			Expect(err).Should(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("JQPatched"))
		})

		It("should successfully MergePatch an object", func() {
			mergePatchYaml := `---
data:
  firstField: "mergePatched"
`
			var mergePatch map[string]interface{}
			err := yaml.Unmarshal([]byte(mergePatchYaml), &mergePatch)
			Expect(err).To(Succeed())

			mergePatchJson, err := json.Marshal(mergePatch)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(object_patch.NewMergePatchOperation(mergePatchJson, testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("mergePatched"))
		})

		It("should successfully JSONPatch an object", func() {
			err := ObjectPatcher.ExecuteOperation(object_patch.NewJSONPatchOperation(
				[]byte(`[{ "op": "replace", "path": "/data/firstField", "value": "jsonPatched"}]`),
				testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(context.TODO(), testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("jsonPatched"))
		})
	})
})

func ensureNamespace(name string) error {
	ns := &corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace"}, ObjectMeta: metav1.ObjectMeta{Name: name}}

	unstructuredNS, err := generateUnstructured(ns)
	if err != nil {
		panic(err)
	}

	return ObjectPatcher.ExecuteOperation(object_patch.NewCreateOperation(unstructuredNS, object_patch.UpdateIfExists()))
}

func ensureTestObject(namespace string, obj interface{}) error {
	unstructuredObj, err := generateUnstructured(obj)
	if err != nil {
		panic(err)
	}

	return ObjectPatcher.ExecuteOperation(object_patch.NewCreateOperation(unstructuredObj, object_patch.UpdateIfExists()))
}

func removeNamespace(name string) error {
	return ObjectPatcher.ExecuteOperation(object_patch.NewDeleteOperation("", "Namespace", "", name))
}

func generateUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	var (
		unstructuredCM = &unstructured.Unstructured{}
		err            error
	)

	unstructuredCM.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)

	return unstructuredCM, err
}

func randomSuffix() string {
	return uuid.NewV4().String()[0:8]
}
