package executor

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/app"
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
		app.LogProxyHookJSON = true

		cmd := exec.Command("echo", `{"foo": "baz"}`)

		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"fatal","msg":"hook result","hook":{"foo":"baz"},"output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("not json log", func(t *testing.T) {
		app.LogProxyHookJSON = false
		cmd := exec.Command("echo", `foobar`)

		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"info","msg":"foobar","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("long file must be truncated", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "testjson-*.json")
		require.NoError(t, err)

		defer os.RemoveAll(f.Name())

		_, _ = io.WriteString(f, `{"foo": "`+randStringRunes(1024*1024)+`"}`)

		app.LogProxyHookJSON = true
		cmd := exec.Command("cat", f.Name())

		_, err = RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)

		reg := regexp.MustCompile(`{"level":"fatal","msg":"hook result","hook":{"truncated":".*:truncated"},"output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"`)
		assert.Regexp(t, reg, buf.String())

		buf.Reset()
	})

	t.Run("long file non json must be truncated", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "testjson-*.json")
		require.NoError(t, err)

		defer os.RemoveAll(f.Name())

		_, _ = io.WriteString(f, `result `+randStringRunes(1024*1024))

		app.LogProxyHookJSON = false
		cmd := exec.Command("cat", f.Name())

		_, err = RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)

		reg := regexp.MustCompile(`{"level":"info","msg":"result .*:truncated","output":"stdout","time":"2006-01-02T15:04:05Z"`)
		assert.Regexp(t, reg, buf.String())

		buf.Reset()
	})

	t.Run("invalid json structure", func(t *testing.T) {
		logger.SetLevel(log.LevelDebug)
		app.LogProxyHookJSON = true
		cmd := exec.Command("echo", `["a","b","c"]`)
		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"debug","msg":"Executing command 'echo [\"a\",\"b\",\"c\"]' in '' dir","source":"executor/executor.go:43","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"debug","msg":"json log line not map[string]interface{}","source":"executor/executor.go:111","line":["a","b","c"],"output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"info","msg":"[\"a\",\"b\",\"c\"]\n","source":"executor/executor.go:114","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline", func(t *testing.T) {
		logger.SetLevel(log.LevelInfo)
		app.LogProxyHookJSON = true
		cmd := exec.Command("echo", `
{"a":"b",
"c":"d"}
`)
		_, err := RunAndLogLines(cmd, map[string]string{"foor": "baar"}, logger)
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"fatal","msg":"hook result","hook":{"a":"b","c":"d"},"output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline non json", func(t *testing.T) {
		app.LogProxyHookJSON = false
		cmd := exec.Command("echo", `
a b
c d
`)
		_, err := RunAndLogLines(cmd, map[string]string{"foor": "baar"}, logger)
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"a b","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n"+
			`{"level":"info","msg":"c d","output":"stdout","time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline json", func(t *testing.T) {
		app.LogProxyHookJSON = true
		cmd := exec.Command("echo", `{
"a":"b",
"c":"d"
}`)
		_, err := RunAndLogLines(cmd, map[string]string{"foor": "baar"}, logger)
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"fatal","msg":"hook result","hook":{"a":"b","c":"d"},"output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
