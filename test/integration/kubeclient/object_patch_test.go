package kubeclient

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

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

func TestObjectPatch(t *testing.T) {
	RunIntegrationSuite(t, "kube client suite", "kube-client-test")
}

var _ = Describe("Kubernetes API object patching", func() {
	Context("creating an object", func() {
		var (
			testCM         *corev1.ConfigMap
			unstructuredCM *unstructured.Unstructured
		)

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = creationNamespace

			var err error
			unstructuredCM, err = generateUnstructured(testCM)
			Expect(err).To(Succeed())

			Expect(ensureNamespace(creationNamespace)).To(Succeed())
			Expect(ensureTestObject(creationNamespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(creationNamespace))
		})

		It("should fail to Create() an object if it already exists", func() {
			err := ObjectPatcher.CreateObject(unstructuredCM, "")
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

			err = ObjectPatcher.CreateOrUpdateObject(unstructuredNewTestCM, "")
			Expect(err).To(Succeed())

			cm, err := KubeClient.CoreV1().ConfigMaps(creationNamespace).Get(newTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(cm.Data).To(Equal(newTestCM.Data))
		})

		It("should successfully CreateOrUpdate() an object if id does not yet exist", func() {
			separateTestCM := testCM.DeepCopy()
			separateTestCM.Name = "separate-test"

			unstructuredSeparateTestCM, err := generateUnstructured(separateTestCM)
			Expect(err).To(Succeed())

			err = ObjectPatcher.CreateOrUpdateObject(unstructuredSeparateTestCM, "")
			Expect(err).To(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(creationNamespace).Get(separateTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
		})
	})

	Context("deleting an object", func() {
		var (
			testCM *corev1.ConfigMap
		)

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = deletionNamespace

			Expect(ensureNamespace(deletionNamespace)).To(Succeed())
			Expect(ensureTestObject(deletionNamespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(deletionNamespace))
		})

		It("should successfully delete an object", func() {
			err := ObjectPatcher.DeleteObjectInBackground(testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name, "")
			Expect(err).Should(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(testCM.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("patching an object", func() {
		var (
			testCM *corev1.ConfigMap
		)

		BeforeEach(func() {
			testCM = testObject.DeepCopy()
			testCM.Namespace = patchNamespace

			Expect(ensureNamespace(patchNamespace)).To(Succeed())
			Expect(ensureTestObject(patchNamespace, testCM)).To(Succeed())
		})

		AfterEach(func() {
			Expect(removeNamespace(patchNamespace))
		})

		It("should successfully JQPatch an object", func() {
			err := ObjectPatcher.JQPatchObject(`.data.firstField = "JQPatched"`, testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name, "")
			Expect(err).Should(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(testCM.Name, metav1.GetOptions{})
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

			err = ObjectPatcher.MergePatchObject(mergePatchJson, testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name, "")
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("mergePatched"))
		})

		It("should successfully MergePatch an object", func() {
			err := ObjectPatcher.JSONPatchObject([]byte(`[{ "op": "replace", "path": "/data/firstField", "value": "jsonPatched"}]`),
				testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name, "")
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(testCM.Name, metav1.GetOptions{})
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

	return ObjectPatcher.CreateOrUpdateObject(unstructuredNS, "")
}

func ensureTestObject(namespace string, obj interface{}) error {
	unstructuredObj, err := generateUnstructured(obj)
	if err != nil {
		panic(err)
	}

	return ObjectPatcher.CreateOrUpdateObject(unstructuredObj, "")
}

func removeNamespace(name string) error {
	return ObjectPatcher.DeleteObject("", "Namespace", "", name, "")
}

func generateUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	var (
		unstructuredCM = &unstructured.Unstructured{}
		err            error
	)

	unstructuredCM.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)

	return unstructuredCM, err
}
