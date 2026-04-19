package conversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestExtractAPIVersions_Single(t *testing.T) {
	objs := []runtime.RawExtension{
		{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Foo"}`)},
	}

	versions := ExtractAPIVersions(objs)
	assert.Equal(t, []string{"example.com/v1"}, versions)
}

func TestExtractAPIVersions_Multiple(t *testing.T) {
	objs := []runtime.RawExtension{
		{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Foo"}`)},
		{Raw: []byte(`{"apiVersion":"example.com/v2","kind":"Foo"}`)},
		{Raw: []byte(`{"apiVersion":"example.com/v3","kind":"Foo"}`)},
	}

	versions := ExtractAPIVersions(objs)
	assert.Len(t, versions, 3)
	assert.Contains(t, versions, "example.com/v1")
	assert.Contains(t, versions, "example.com/v2")
	assert.Contains(t, versions, "example.com/v3")
}

func TestExtractAPIVersions_Deduplicated(t *testing.T) {
	objs := []runtime.RawExtension{
		{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Foo"}`)},
		{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"Bar"}`)},
		{Raw: []byte(`{"apiVersion":"example.com/v2","kind":"Foo"}`)},
	}

	versions := ExtractAPIVersions(objs)
	assert.Len(t, versions, 2)
	assert.Contains(t, versions, "example.com/v1")
	assert.Contains(t, versions, "example.com/v2")
}

func TestExtractAPIVersions_Empty(t *testing.T) {
	versions := ExtractAPIVersions(nil)
	assert.Empty(t, versions)
}

func TestExtractAPIVersions_InvalidJSON(t *testing.T) {
	objs := []runtime.RawExtension{
		{Raw: []byte(`not json`)},
	}

	versions := ExtractAPIVersions(objs)
	assert.Len(t, versions, 1)
	assert.Contains(t, versions, "")
}

func TestDetectCrdName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/mycrds.example.com", "mycrds.example.com"},
		{"/", ""},
		{"", ""},
		{"/foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, detectCrdName(tt.path))
		})
	}
}
