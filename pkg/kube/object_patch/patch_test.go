package object_patch

import (
	"io/ioutil"
	"testing"
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
			o := &ObjectPatcher{kubeClient: nil}
			testSpecs := mustReadFile(tt.testFilePath)

			_, err := o.ParseSpecs(testSpecs)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
