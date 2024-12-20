package executor

import (
	"bytes"
	"io"
	"math/rand/v2"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAndLogLines(t *testing.T) {
	loggerOpts := log.Options{
		TimeFunc: func(_ time.Time) time.Time {
			parsedTime, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
			if err != nil {
				assert.NoError(t, err)
			}

			return parsedTime
		},
	}
	logger := log.NewLogger(loggerOpts)
	logger.SetLevel(log.LevelInfo)

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	t.Run("simple log", func(t *testing.T) {
		ex := NewExecutor("", "echo", []string{`{"foo": "baz"}`}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"a": "b"})
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","a":"b","hook_foo":"baz","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("not json log", func(t *testing.T) {
		ex := NewExecutor("", "echo", []string{"foobar"}, []string{}).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"a": "b"})
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"info","msg":"foobar","a":"b","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("long file must be truncated", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "testjson-*.json")
		require.NoError(t, err)

		defer os.RemoveAll(f.Name())

		_, _ = io.WriteString(f, `{"foo": "`+randStringRunes(1024*1024)+`"}`)

		ex := NewExecutor("", "cat", []string{f.Name()}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err = ex.RunAndLogLines(map[string]string{"a": "b"})
		assert.NoError(t, err)

		reg := regexp.MustCompile(`{"level":"info","msg":"hook result","a":"b","hook":{"truncated":".*:truncated"},"output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"`)
		assert.Regexp(t, reg, buf.String())

		buf.Reset()
	})

	t.Run("long file non json must be truncated", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "testjson-*.json")
		require.NoError(t, err)

		defer os.RemoveAll(f.Name())

		_, _ = io.WriteString(f, `result `+randStringRunes(1024*1024))

		ex := NewExecutor("", "cat", []string{f.Name()}, []string{}).
			WithLogger(logger)

		_, err = ex.RunAndLogLines(map[string]string{"a": "b"})
		assert.NoError(t, err)

		reg := regexp.MustCompile(`{"level":"info","msg":"result .*:truncated","a":"b","output":"stdout","time":"2006-01-02T15:04:05Z"`)
		assert.Regexp(t, reg, buf.String())

		buf.Reset()
	})

	t.Run("invalid json structure", func(t *testing.T) {
		logger.SetLevel(log.LevelDebug)

		ex := NewExecutor("", "echo", []string{`["a","b","c"]`}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"a": "b"})
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"debug","msg":"json log line not map[string]interface{}","source":"executor/executor.go:200","a":"b","line":["a","b","c"],"output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"info","msg":"[\"a\",\"b\",\"c\"]\n","source":"executor/executor.go:203","a":"b","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")
		buf.Reset()
	})

	t.Run("multiline", func(t *testing.T) {
		logger.SetLevel(log.LevelInfo)
		arg := `
{"a":"b",
"c":"d"}
`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline non json", func(t *testing.T) {
		arg := `
a b
c d
`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"a b","foor":"baar","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"info","msg":"c d","foor":"baar","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline non json with json proxy on", func(t *testing.T) {
		arg := `
a b
c d
`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"a b","foor":"baar","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"info","msg":"c d","foor":"baar","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline json", func(t *testing.T) {
		arg := `{
"a":"b",
"c":"d"
}`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("input json nest", func(t *testing.T) {
		arg := `{"level":"info","msg":"hook result","foor":"baar","a":"b","c":"d","time":"2024-01-02T15:04:05Z"}`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","hook_foor":"baar","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.IntN(len(letterRunes))]
	}
	return string(b)
}
