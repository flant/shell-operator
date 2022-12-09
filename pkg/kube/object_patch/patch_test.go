package object_patch

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/flant/kube-client/fake"
	"github.com/flant/kube-client/manifest"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func mustReadFile(t *testing.T, filePath string) []byte {
	t.Helper()
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return content
}

func Test_ParseOperations(t *testing.T) {
	const (
		shouldNotBeError = false
		shouldBeError    = true
	)

	tests := []struct {
		name         string
		testFilePath string
		expectError  bool
	}{
		{
			"valid create",
			"testdata/serialized_operations/valid_create.yaml",
			shouldNotBeError,
		},
		{
			"invalid create",
			"testdata/serialized_operations/invalid_create.yaml",
			shouldBeError,
		},
		{
			"valid delete",
			"testdata/serialized_operations/valid_delete.yaml",
			shouldNotBeError,
		},
		{
			"invalid delete",
			"testdata/serialized_operations/invalid_delete.yaml",
			shouldBeError,
		},
		{
			"valid patch",
			"testdata/serialized_operations/valid_patch.yaml",
			shouldNotBeError,
		},
		{
			"invalid patch",
			"testdata/serialized_operations/invalid_patch.yaml",
			shouldBeError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSpecs := mustReadFile(t, tt.testFilePath)

			_, err := ParseOperations(testSpecs)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_PatchOperations(t *testing.T) {
	const (
		namespace   = "default"
		name        = "testcm"
		missingName = "missing-object"
		configMap   = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: testcm
data:
  foo: "bar"
`
		newField         = "baz"
		newValue         = "quux"
		shouldNotAdd     = false
		shouldAdd        = true
		shouldNotBeError = false
		shouldBeError    = true
	)

	// Filter func to add a new field.
	var filter = func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		res := u.DeepCopy()
		data := res.Object["data"].(map[string]interface{})
		data[newField] = newValue
		res.Object["data"] = data
		return res, nil
	}

	tests := []struct {
		name        string
		fn          func(patcher *ObjectPatcher) error
		expectAdd   bool
		expectError bool
	}{
		{
			"merge patch",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewMergePatchOperation(
					fmt.Sprintf(`{"data":{"%s":"%s"}}`, newField, newValue),
					"v1", "ConfigMap", namespace, name,
				))
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"merge patch a missing object",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewMergePatchOperation(
					fmt.Sprintf(`{"data":{"%s":"%s"}}`, newField, newValue),
					"v1", "ConfigMap", namespace, missingName,
				))
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"merge patch with ignoreMissingObject",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewMergePatchOperation(
					fmt.Sprintf(`{"data":{"%s":"%s"}}`, newField, newValue),
					"v1", "ConfigMap", namespace, missingName,
					IgnoreMissingObject(),
				))
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"merge patch using map",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewMergePatchOperation(
					map[string]interface{}{
						"data": map[string]interface{}{
							newField: newValue,
						},
					},
					"v1", "ConfigMap", namespace, name,
				))
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"merge patch via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: MergePatch
kind: ConfigMap
namespace: %s
name: %s
mergePatch:
  data:
    %s: "%s"
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"merge patch a missing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: MergePatch
kind: ConfigMap
namespace: %s
name: %s
mergePatch:
  data:
    %s: "%s"
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"merge patch with ignoreMissingObject via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: MergePatch
kind: ConfigMap
namespace: %s
name: %s
ignoreMissingObject: true
mergePatch:
  data:
    %s: "%s"
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"merge patch via string in YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: MergePatch
kind: ConfigMap
namespace: %s
name: %s
mergePatch: |
  data:
    %s: "%s"
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"merge patch via stringified JSON in YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: MergePatch
kind: ConfigMap
namespace: %s
name: %s
mergePatch: |
  {"data":{"%s":"%s"}}
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"json patch",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewJSONPatchOperation(
					// [{ "op": "replace", "path": "/data/firstField", "value": "jsonPatched"}]
					fmt.Sprintf(`[{ "op": "add", "path": "/data/%s", "value": "%s"}]`, newField, newValue),
					"v1", "ConfigMap", namespace, name,
				))
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"json patch a missing object",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewJSONPatchOperation(
					fmt.Sprintf(`[{ "op": "add", "path": "/data/%s", "value": "%s"}]`, newField, newValue),
					"v1", "ConfigMap", namespace, missingName,
				))
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"json patch with ignoreMissingObject",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewJSONPatchOperation(
					fmt.Sprintf(`[{ "op": "add", "path": "/data/%s", "value": "%s"}]`, newField, newValue),
					"v1", "ConfigMap", namespace, missingName,
					IgnoreMissingObject(),
				))
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"json patch via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JSONPatch
kind: ConfigMap
namespace: %s
name: %s
jsonPatch:
  - op: add
    path: /data/%s
    value: %s
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"json patch a missing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JSONPatch
kind: ConfigMap
namespace: %s
name: %s
jsonPatch:
  - op: add
    path: /data/%s
    value: %s
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"json patch with ignoreMissingObject via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JSONPatch
kind: ConfigMap
namespace: %s
name: %s
ignoreMissingObject: true
jsonPatch:
  - op: add
    path: /data/%s
    value: %s
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"json patch via stringified JSON in YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JSONPatch
kind: ConfigMap
namespace: %s
name: %s
jsonPatch: |
  [{"op":"add", "path":"/data/%s", "value":"%s"}]
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"filter patch",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewFilterPatchOperation(
					filter,
					"v1", "ConfigMap", namespace, name,
				))
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"filter patch missing object",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewFilterPatchOperation(
					filter,
					"v1", "ConfigMap", namespace, missingName,
				))
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"filter patch with ignoreMissingObject",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewFilterPatchOperation(
					filter,
					"v1", "ConfigMap", namespace, missingName,
					IgnoreMissingObject(),
				))
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"JQ patch via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JQPatch
kind: ConfigMap
namespace: %s
name: %s
jqFilter: |
  .data.%s = "%s"
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"JQ patch missing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JQPatch
kind: ConfigMap
namespace: %s
name: %s
jqFilter: |
  .data.%s = "%s"
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldBeError,
		},
		{
			"JQ patch with ignoreMissingObject via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: JQPatch
kind: ConfigMap
namespace: %s
name: %s
ignoreMissingObject: true
jqFilter: |
  .data.%s = "%s"
`, namespace, missingName, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"update existing object",
			func(patcher *ObjectPatcher) error {
				obj := manifest.MustFromYAML(fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: %s
  name: %s
data:
  foo: "bar"
  %s: "%s"
`, namespace, name, newField, newValue)).Unstructured()
				return patcher.ExecuteOperation(NewCreateOperation(obj, UpdateIfExists()))
			},
			shouldAdd,
			shouldNotBeError,
		},
		{
			"update existing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: CreateOrUpdate
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: "bar"
    %s: "%s"
`, namespace, name, newField, newValue)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldAdd,
			shouldNotBeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare fake cluster: create a Namespace and a ConfigMap.
			cluster := newFakeClusterWithNamespaceAndObjects(t, namespace, configMap)

			// Apply MergePatch: add a new field in data section.
			patcher := NewObjectPatcher(cluster.Client)

			err := tt.fn(patcher)

			// Check error expectation.
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Fetch updated object and check fields in data section.
			cmObj := new(v1.ConfigMap)
			fetchObject(t, cluster, namespace, configMap, cmObj)

			require.Equal(t, "bar", cmObj.Data["foo"])
			if tt.expectAdd {
				require.Equal(t, newValue, cmObj.Data[newField])
			} else {
				require.NotContains(t, cmObj.Data, newField)
			}
		})
	}
}

func Test_CreateOperations(t *testing.T) {
	const (
		namespace         = "default"
		existingName      = "testcm"
		existingConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: testcm
data:
  foo: "bar"
`
		newName      = "newtestcm"
		newConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: newtestcm
data:
  foo: "bar"
`
		shouldNotCreateNew = false
		shouldCreateNew    = true
		shouldNotBeError   = false
		shouldBeError      = true
	)

	tests := []struct {
		name            string
		fn              func(patcher *ObjectPatcher) error
		expectNewExists bool
		expectError     bool
	}{
		{
			"create new object",
			func(patcher *ObjectPatcher) error {
				obj := manifest.MustFromYAML(newConfigMap).Unstructured()
				return patcher.ExecuteOperation(NewCreateOperation(obj))
			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create new object ignore existing object",
			func(patcher *ObjectPatcher) error {
				obj := manifest.MustFromYAML(newConfigMap).Unstructured()
				return patcher.ExecuteOperation(NewCreateOperation(obj, IgnoreIfExists()))
			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create existing object",
			func(patcher *ObjectPatcher) error {
				obj := manifest.MustFromYAML(existingConfigMap).Unstructured()
				return patcher.ExecuteOperation(NewCreateOperation(obj))
			},
			shouldNotCreateNew,
			shouldBeError,
		},
		{
			"create ignore existing object",
			func(patcher *ObjectPatcher) error {
				obj := manifest.MustFromYAML(existingConfigMap).Unstructured()
				return patcher.ExecuteOperation(NewCreateOperation(obj, IgnoreIfExists()))
			},
			shouldNotCreateNew,
			shouldNotBeError,
		},
		{
			"create new object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Create
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: bar
`, namespace, newName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create existing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Create
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: bar
`, namespace, existingName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)

			},
			shouldNotCreateNew,
			shouldBeError,
		},
		{
			"create ignore existing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: CreateIfNotExists
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: bar
`, namespace, existingName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)

			},
			shouldNotCreateNew,
			shouldNotBeError,
		},
		{
			"create or update new object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: CreateOrUpdate
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: bar
`, namespace, newName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create new object via string in YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Create
object: |
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: %s
    name: %s
  data:
    foo: bar
`, namespace, newName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)

			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create new object via stringified JSON in YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Create
object: |
  {"apiVersion":"v1", "kind":"ConfigMap", "metadata": {
    "namespace":"%s", "name":"%s"},
  "data":{"foo":"bar"}}
`, namespace, newName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)

			},
			shouldCreateNew,
			shouldNotBeError,
		},
		{
			"create new object via stringified JSON in JSON spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
{"operation": "Create",
"object": "{
  \"apiVersion\":\"v1\", \"kind\":\"ConfigMap\",
  \"metadata\": {\"namespace\":\"%s\", \"name\":\"%s\"},
  \"data\":{\"foo\":\"bar\"}}
"}
`, namespace, newName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldCreateNew,
			shouldNotBeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare fake cluster: create a Namespace and a ConfigMap.
			cluster := newFakeClusterWithNamespaceAndObjects(t, namespace, existingConfigMap)

			// Apply MergePatch: add a new field in data section.
			patcher := NewObjectPatcher(cluster.Client)

			err := tt.fn(patcher)

			// Check error expectation.
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Check if new object is created.
			exists := existObject(t, cluster, namespace, newConfigMap)

			if tt.expectNewExists {
				require.True(t, exists)
			} else {
				require.False(t, exists)
			}
		})
	}
}

func Test_DeleteOperations(t *testing.T) {
	const (
		namespace         = "default"
		existingName      = "testcm"
		existingConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: testcm
data:
  foo: "bar"
`
		missingName = "missing-object"

		shouldNotDelete  = false
		shouldDelete     = true
		shouldNotBeError = false
		shouldBeError    = true
	)

	tests := []struct {
		name          string
		fn            func(patcher *ObjectPatcher) error
		expectDeleted bool
		expectError   bool
	}{
		{
			"delete existing object",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewDeleteOperation("", "ConfigMap", namespace, existingName))
			},
			shouldDelete,
			shouldNotBeError,
		},
		{
			"delete missing object",
			func(patcher *ObjectPatcher) error {
				return patcher.ExecuteOperation(NewDeleteOperation("", "ConfigMap", namespace, missingName))
			},
			shouldNotDelete,
			shouldNotBeError,
		},
		{
			"delete existing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Delete
kind: ConfigMap
namespace: %s
name: %s
`, namespace, existingName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldDelete,
			shouldNotBeError,
		},
		{
			"delete missing object via YAML spec",
			func(patcher *ObjectPatcher) error {
				operations, err := ParseOperations([]byte(fmt.Sprintf(`
operation: Delete
kind: ConfigMap
namespace: %s
name: %s
`, namespace, missingName)))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotDelete,
			shouldNotBeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare fake cluster: create a Namespace and a ConfigMap.
			cluster := newFakeClusterWithNamespaceAndObjects(t, namespace, existingConfigMap)

			// Apply MergePatch: add a new field in data section.
			patcher := NewObjectPatcher(cluster.Client)

			err := tt.fn(patcher)

			// Check error expectation.
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Check if new object is created.
			exists := existObject(t, cluster, namespace, existingConfigMap)

			if tt.expectDeleted {
				require.False(t, exists)
			} else {
				require.True(t, exists)
			}
		})
	}
}

func newFakeClusterWithNamespaceAndObjects(t *testing.T, ns string, objects ...string) *fake.Cluster {
	t.Helper()

	// Prepare fake cluster: create a Namespace and a ConfigMap.
	cluster := fake.NewFakeCluster(fake.ClusterVersionV119)
	cluster.CreateNs(ns)

	for _, object := range objects {
		mft := manifest.MustFromYAML(object)
		err := cluster.Create(ns, mft)
		require.NoError(t, err)
	}

	return cluster
}

func fetchObject(t *testing.T, cluster *fake.Cluster, ns string, objYAML string, object interface{}) {
	t.Helper()
	mft, err := manifest.NewFromYAML(objYAML)
	require.NoError(t, err)

	gvk := cluster.MustFindGVR(mft.ApiVersion(), mft.Kind())
	obj, err := cluster.Client.Dynamic().Resource(*gvk).Namespace(ns).Get(context.TODO(), mft.Name(), metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, obj)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), object)
	require.NoError(t, err)
}

func existObject(t *testing.T, cluster *fake.Cluster, ns string, objYAML string) bool {
	mft := manifest.MustFromYAML(objYAML)
	gvk := cluster.MustFindGVR(mft.ApiVersion(), mft.Kind())
	obj, err := cluster.Client.Dynamic().Resource(*gvk).Namespace(ns).Get(context.TODO(), mft.Name(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)
	return obj != nil
}
