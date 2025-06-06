//go:build integration
// +build integration

package kubeclient_test

import (
	"context"
	"encoding/json"
	"time"

	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	. "github.com/flant/shell-operator/test/integration/suite"
	uuid "github.com/gofrs/uuid/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
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

		It("should fail to Create() an object if it already exists", func(ctx context.Context) {
			err := ObjectPatcher.ExecuteOperation(objectpatch.NewCreateOperation(unstructuredCM))
			Expect(err).To(Not(Succeed()))
		}, SpecTimeout(10*time.Second))

		It("should should successfully CreateOrUpdate() an object even if it already exists", func(ctx context.Context) {
			newTestCM := testCM.DeepCopy()
			newTestCM.Data = map[string]string{
				"firstField":  "test3",
				"secondField": "test4",
			}

			unstructuredNewTestCM, err := generateUnstructured(newTestCM)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(objectpatch.NewCreateOrUpdateOperation(unstructuredNewTestCM))
			Expect(err).To(Succeed())

			cm, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, newTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(cm.Data).To(Equal(newTestCM.Data))
		}, SpecTimeout(10*time.Second))

		It("should successfully CreateOrUpdate() an object if id does not yet exist", func(ctx context.Context) {
			separateTestCM := testCM.DeepCopy()
			separateTestCM.Name = "separate-test"

			unstructuredSeparateTestCM, err := generateUnstructured(separateTestCM)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(objectpatch.NewCreateOrUpdateOperation(unstructuredSeparateTestCM))
			Expect(err).To(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, separateTestCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
		}, SpecTimeout(10*time.Second))
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

		It("should successfully delete an object", func(ctx context.Context) {
			err := ObjectPatcher.ExecuteOperation(objectpatch.NewDeleteInBackgroundOperation(
				testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).Should(Succeed())

			_, err = KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, testCM.Name, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		}, SpecTimeout(10*time.Second))

		It("should successfully delete an object if it doesn't exist", func(ctx context.Context) {
			err := ObjectPatcher.ExecuteOperation(objectpatch.NewDeleteOperation(testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).Should(Succeed())
		}, SpecTimeout(10*time.Second))
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

		It("should successfully JQPatch an object", func(ctx context.Context) {
			err := ObjectPatcher.ExecuteOperation(objectpatch.NewFromOperationSpec(objectpatch.OperationSpec{
				Operation:  objectpatch.JQPatch,
				ApiVersion: testCM.APIVersion,
				Kind:       testCM.Kind,
				Namespace:  testCM.Namespace,
				Name:       testCM.Name,
				JQFilter:   `.data.firstField = "JQPatched"`,
			}))
			Expect(err).Should(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("JQPatched"))
		}, SpecTimeout(10*time.Second))

		It("should successfully MergePatch an object", func(ctx context.Context) {
			mergePatchYaml := `---
data:
  firstField: "mergePatched"
`
			var mergePatch map[string]interface{}
			err := yaml.Unmarshal([]byte(mergePatchYaml), &mergePatch)
			Expect(err).To(Succeed())

			mergePatchJson, err := json.Marshal(mergePatch)
			Expect(err).To(Succeed())

			err = ObjectPatcher.ExecuteOperation(objectpatch.NewMergePatchOperation(mergePatchJson, testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("mergePatched"))
		}, SpecTimeout(10*time.Second))

		It("should successfully JSONPatch an object", func(ctx context.Context) {
			err := ObjectPatcher.ExecuteOperation(objectpatch.NewJSONPatchOperation(
				[]byte(`[{ "op": "replace", "path": "/data/firstField", "value": "jsonPatched"}]`),
				testCM.APIVersion, testCM.Kind, testCM.Namespace, testCM.Name))
			Expect(err).To(Succeed())

			existingCM, err := KubeClient.CoreV1().ConfigMaps(testCM.Namespace).Get(ctx, testCM.Name, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(existingCM.Data["firstField"]).To(Equal("jsonPatched"))
		}, SpecTimeout(10*time.Second))
	})
})

func ensureNamespace(name string) error {
	ns := &corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace"}, ObjectMeta: metav1.ObjectMeta{Name: name}}

	unstructuredNS, err := generateUnstructured(ns)
	if err != nil {
		panic(err)
	}

	return ObjectPatcher.ExecuteOperation(objectpatch.NewCreateOrUpdateOperation(unstructuredNS))
}

func ensureTestObject(_ string, obj interface{}) error {
	unstructuredObj, err := generateUnstructured(obj)
	if err != nil {
		panic(err)
	}

	return ObjectPatcher.ExecuteOperation(objectpatch.NewCreateOrUpdateOperation(unstructuredObj))
}

func removeNamespace(name string) error {
	return ObjectPatcher.ExecuteOperation(objectpatch.NewDeleteOperation("", "Namespace", "", name))
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
	return uuid.Must(uuid.NewV4()).String()[0:8]
}
