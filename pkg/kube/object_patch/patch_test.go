package object_patch

import (
	"io/ioutil"
	"testing"

	. "github.com/onsi/gomega"
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

			_, err := ParseSpecs(testSpecs)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
