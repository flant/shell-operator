package conversion

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestResponseFromBytes_Success(t *testing.T) {
	data := []byte(`{"failedMessage":"","convertedObjects":[{"raw":"eyJhcGlWZXJzaW9uIjoiZXhhbXBsZS5jb20vdjEifQ=="}]}`)

	resp, err := ResponseFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.FailedMessage)
	assert.Len(t, resp.ConvertedObjects, 1)
}

func TestResponseFromBytes_WithFailedMessage(t *testing.T) {
	data := []byte(`{"failedMessage":"conversion not supported"}`)

	resp, err := ResponseFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "conversion not supported", resp.FailedMessage)
}

func TestResponseFromBytes_EmptyInput(t *testing.T) {
	// ResponseFromBytes does not have an empty guard (ResponseFromFile does),
	// so decoding empty input returns an EOF error.
	resp, err := ResponseFromBytes([]byte{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestResponseFromBytes_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)

	resp, err := ResponseFromBytes(data)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestResponseFromReader(t *testing.T) {
	input := `{"failedMessage":"","convertedObjects":[]}`
	resp, err := ResponseFromReader(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.FailedMessage)
	assert.Empty(t, resp.ConvertedObjects)
}

func TestResponse_Dump_WithFailedMessage(t *testing.T) {
	resp := &Response{FailedMessage: "something broke"}
	dump := resp.Dump()
	assert.Contains(t, dump, "failedMessage=something broke")
	assert.Contains(t, dump, "conversion.Response(")
}

func TestResponse_Dump_WithConvertedObjects(t *testing.T) {
	resp := &Response{
		ConvertedObjects: make([]runtime.RawExtension, 3),
	}
	dump := resp.Dump()
	assert.Contains(t, dump, "convertedObjects.len=3")
}

func TestResponse_Dump_Empty(t *testing.T) {
	resp := &Response{}
	dump := resp.Dump()
	assert.Equal(t, "conversion.Response()", dump)
}
