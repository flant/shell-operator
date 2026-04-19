package debug

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStubRequest(path string) *http.Request {
	if path == "" {
		path = "/debug/test"
	}
	return httptest.NewRequest(http.MethodGet, path, nil)
}

func TestTransformUsingFormat_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, map[string]int{"count": 42}, "json")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"count":42`)
}

func TestTransformUsingFormat_YAML(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, map[string]string{"name": "test"}, "yaml")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "name: test")
}

func TestTransformUsingFormat_Text_String(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, "hello world", "text")
	require.NoError(t, err)
	assert.Equal(t, "hello world", buf.String())
}

func TestTransformUsingFormat_Text_Bytes(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, []byte("raw bytes"), "text")
	require.NoError(t, err)
	assert.Equal(t, "raw bytes", buf.String())
}

func TestTransformUsingFormat_Text_Stringer(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, stringerVal{s: "from-stringer"}, "text")
	require.NoError(t, err)
	assert.Equal(t, "from-stringer", buf.String())
}

type stringerVal struct{ s string }

func (sv stringerVal) String() string { return sv.s }

func TestTransformUsingFormat_Text_NonStringFallsBackToJSON(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, map[string]int{"x": 1}, "text")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"x":1`)
}

func TestTransformUsingFormat_UnknownFormatFallsBackToJSON(t *testing.T) {
	var buf bytes.Buffer
	err := transformUsingFormat(&buf, map[string]int{"y": 2}, "xml")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"y":2`)
}

func TestFormatFromRequest_Default(t *testing.T) {
	assert.Equal(t, "text", FormatFromRequest(newStubRequest("")))
}

func TestBadRequestError(t *testing.T) {
	err := &BadRequestError{Msg: "missing param"}
	assert.Equal(t, "missing param", err.Error())
}

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Msg: "not found"}
	assert.Equal(t, "not found", err.Error())
}
