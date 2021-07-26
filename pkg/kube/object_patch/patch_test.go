package object_patch

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/flant/kube-client/fake"
	"github.com/flant/kube-client/manifest"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: Kubernetes API interaction is tested only in in the integration tests. Unit tests with the FakeClient would be faster.

func mustReadFile(filePath string) []byte {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	return contents
}

func TestParseSpecs(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name         string
		testFilePath string
		wantErr      bool
	}{
		{
			name:         "valid create",
			testFilePath: "testdata/serialized_operations/valid_create.yaml",
			wantErr:      false,
		},
		{
			name:         "invalid create",
			testFilePath: "testdata/serialized_operations/invalid_create.yaml",
			wantErr:      true,
		},
		{
			name:         "valid delete",
			testFilePath: "testdata/serialized_operations/valid_delete.yaml",
			wantErr:      false,
		},
		{
			name:         "invalid delete",
			testFilePath: "testdata/serialized_operations/invalid_delete.yaml",
			wantErr:      true,
		},
		{
			name:         "valid patch",
			testFilePath: "testdata/serialized_operations/valid_patch.yaml",
			wantErr:      false,
		},
		{
			name:         "invalid patch",
			testFilePath: "testdata/serialized_operations/invalid_patch.yaml",
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSpecs := mustReadFile(tt.testFilePath)

			_, err := ParseOperations(testSpecs)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func Test_addFieldInConfigMapWithPatchOperation(t *testing.T) {
	const (
		namespace = "default"
		name      = "testcm"
		configMap = `
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
					"v1", "ConfigMap", namespace, "non-exists",
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
					"v1", "ConfigMap", namespace, "non-exists",
					IgnoreMissingObject(),
				))
			},
			shouldNotAdd,
			shouldNotBeError,
		},
		{
			"merge patch via YAML spec",
			func(patcher *ObjectPatcher) error {
				var operations []*Operation
				operations, err := ParseOperations([]byte(`
operation: MergePatch
kind: ConfigMap
namespace: default
name: testcm
mergePatch:
  data:
    baz: "quux"
`))
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
				var operations []*Operation
				operations, err := ParseOperations([]byte(`
operation: MergePatch
kind: ConfigMap
namespace: default
name: testcm-no-object
mergePatch:
  data:
    baz: "quux"
`))
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
				var operations []*Operation
				operations, err := ParseOperations([]byte(`
operation: MergePatch
kind: ConfigMap
namespace: default
name: testcm-no-object
ignoreMissingObject: true
mergePatch:
  data:
    baz: "quux"
`))
				if err != nil {
					return err
				}
				return patcher.ExecuteOperations(operations)
			},
			shouldNotAdd,
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
	obj, err := cluster.Client.Dynamic().Resource(*gvk).Namespace(ns).Get(context.TODO(), "testcm", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, obj)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), object)
	require.NoError(t, err)
}
