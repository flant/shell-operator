package executor

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/unilogger"
)

func TestRunAndLogLines(t *testing.T) {
	logger := unilogger.NewLogger(unilogger.Options{})
	logger.SetLevel(unilogger.LevelInfo)

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	t.Run("simple log", func(t *testing.T) {
		app.LogProxyHookJSON = true
		// time="2023-07-10T18:13:42+04:00" level=fatal msg="{\"a\":\"b\",\"foo\":\"baz\",\"output\":\"stdout\"}" a=b output=stdout proxyJsonLog=true
		cmd := exec.Command("echo", `{"foo": "baz"}`)
		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), `"level":"fatal","msg":"hook result","hook":{"foo":"baz"},"output":"stdout","proxyJsonLog":true`)

		buf.Reset()
	})

	t.Run("not json log", func(t *testing.T) {
		app.LogProxyHookJSON = false
		// time="2023-07-10T18:14:25+04:00" level=info msg=foobar a=b output=stdout
		cmd := exec.Command("echo", `foobar`)
		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		time.Sleep(100 * time.Millisecond)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), `"level":"info","msg":"foobar","output":"stdout"`)

		buf.Reset()
	})

	// TODO: check test
	t.Run("long file", func(t *testing.T) {
		f, err := os.CreateTemp(os.TempDir(), "testjson-*.json")
		require.NoError(t, err)
		defer os.RemoveAll(f.Name())

		_, _ = io.WriteString(f, `{"foo": "`+randStringRunes(1024*1024)+`"}`)

		app.LogProxyHookJSON = true
		cmd := exec.Command("cat", f.Name())
		_, err = RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)
		// assert.Equal(t, buf.String(), `\",\"output\":\"stdout\"}" a=b output=stdout proxyJsonLog=true`)
		assert.Contains(t, buf.String(), `"level":"fatal","msg":"hook result","hook":{"foo":`)

		buf.Reset()
	})

	t.Run("invalid json structure", func(t *testing.T) {
		logger.SetLevel(unilogger.LevelDebug)
		app.LogProxyHookJSON = true
		cmd := exec.Command("echo", `["a","b","c"]`)
		_, err := RunAndLogLines(cmd, map[string]string{"a": "b"}, logger)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), `"level":"debug","msg":"json log line not map[string]interface{}: [a b c]","source":"executor/executor.go:109","output":"stdout"`)

		buf.Reset()
	})

	t.Run("multiline", func(t *testing.T) {
		logger.SetLevel(unilogger.LevelInfo)
		app.LogProxyHookJSON = true
		cmd := exec.Command("echo", `
{"a":"b",
"c":"d"}
`)
		_, err := RunAndLogLines(cmd, map[string]string{"foor": "baar"}, logger)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), `"level":"fatal","msg":"hook result","hook":{"a":"b","c":"d"},"output":"stdout","proxyJsonLog":true`)

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
