package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	json "github.com/flant/shell-operator/pkg/utils/json"
)

func TestRunAndLogLines(t *testing.T) {
	timeFunc := func(_ time.Time) time.Time {
		parsedTime, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
		if err != nil {
			assert.NoError(t, err)
		}

		return parsedTime
	}
	logger := log.NewLogger(log.WithTimeFunc(timeFunc))
	logger.SetLevel(log.LevelInfo)

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	t.Run("simple log", func(t *testing.T) {
		ex := NewExecutor("", "echo", []string{`{"foo": "baz"}`}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"a": "b"})
		assert.NoError(t, err)

		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","a":"b","hook_foo":"baz","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("not json log", func(t *testing.T) {
		ex := NewExecutor("", "echo", []string{"foobar"}, []string{}).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"a": "b"})
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

		_, err = ex.RunAndLogLines(context.Background(), map[string]string{"a": "b"})
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

		_, err = ex.RunAndLogLines(context.Background(), map[string]string{"a": "b"})
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

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"a": "b"})
		assert.NoError(t, err)
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

		var debugLine map[string]interface{}
		err = json.Unmarshal([]byte(lines[0]), &debugLine)
		assert.NoError(t, err)
		assert.Equal(t, "debug", debugLine["level"])
		assert.Equal(t, "json log line not map[string]interface{}", debugLine["msg"])
		assert.Equal(t, []interface{}{"a", "b", "c"}, debugLine["line"])
		assert.Equal(t, "b", debugLine["a"])
		assert.Equal(t, "stdout", debugLine["output"])
		assert.Equal(t, "2006-01-02T15:04:05Z", debugLine["time"])

		var infoLine map[string]interface{}
		err = json.Unmarshal([]byte(lines[1]), &infoLine)
		assert.NoError(t, err)
		assert.Equal(t, "info", infoLine["level"])
		assert.Equal(t, "[\"a\",\"b\",\"c\"]\n", infoLine["msg"])
		assert.Equal(t, "b", infoLine["a"])
		assert.Equal(t, "stdout", infoLine["output"])
		assert.Equal(t, "2006-01-02T15:04:05Z", infoLine["time"])
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

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
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

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
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

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
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

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("input json nest", func(t *testing.T) {
		arg := `{"level":"info","msg":"hook result","foor":"baar","a":"b","c":"d","time":"2024-01-02T15:04:05Z"}`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		assert.Equal(t, buf.String(), `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","hook_foor":"baar","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}`+"\n")

		buf.Reset()
	})

	t.Run("multiline json several logs", func(t *testing.T) {
		logger.SetLevel(log.LevelInfo)
		arg := `
{"a":"b","c":"d"}
{"b":"c","d":"a"}
{"c":"d","a":"b"}
{"d":"a","b":"c"}
plain text
{
"a":"b",
"c":"d"
}`
		ex := NewExecutor("", "echo", []string{arg}, []string{}).
			WithLogProxyHookJSON(true).
			WithLogger(logger)

		_, err := ex.RunAndLogLines(context.Background(), map[string]string{"foor": "baar"})
		assert.NoError(t, err)
		fmt.Println("actual", buf.String())
		expected := ""
		expected += `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}` + "\n"
		expected += `{"level":"info","msg":"hook result","foor":"baar","hook_b":"c","hook_d":"a","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}` + "\n"
		expected += `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}` + "\n"
		expected += `{"level":"info","msg":"hook result","foor":"baar","hook_b":"c","hook_d":"a","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}` + "\n"
		expected += `{"level":"info","msg":"plain text","foor":"baar","output":"stdout","time":"2006-01-02T15:04:05Z"}` + "\n"
		expected += `{"level":"info","msg":"hook result","foor":"baar","hook_a":"b","hook_c":"d","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05Z"}` + "\n"
		assert.Equal(t, expected, buf.String())

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

// newTestRegistry creates a fresh processRegistry for tests and swaps the
// global singleton, returning a cleanup function that restores it.
func newTestRegistry(t *testing.T) *processRegistry {
	t.Helper()

	r := &processRegistry{activePIDs: make(map[int32]struct{})}
	orig := registry
	registry = r
	t.Cleanup(func() { registry = orig })

	return r
}

func TestProcessRegistry_Basic(t *testing.T) {
	r := &processRegistry{activePIDs: make(map[int32]struct{})}

	// Initially empty
	assert.False(t, r.IsActive(1), "IsActive should return false for unknown PID")
	assert.False(t, r.IsActive(12345), "IsActive should return false for unknown PID")

	// Register and check
	r.register(42)
	assert.True(t, r.IsActive(42), "IsActive should return true for registered PID")
	assert.False(t, r.IsActive(43), "IsActive should return false for different PID")

	// Unregister and check
	r.unregister(42)
	assert.False(t, r.IsActive(42), "IsActive should return false after unregister")
}

func TestProcessRegistry_DoubleUnregister(t *testing.T) {
	r := &processRegistry{activePIDs: make(map[int32]struct{})}

	r.register(100)
	r.unregister(100)
	r.unregister(100) // should not panic

	assert.False(t, r.IsActive(100))
}

func TestProcessRegistry_Concurrent(t *testing.T) {
	r := &processRegistry{activePIDs: make(map[int32]struct{})}
	const goroutines = 100
	const pidsPerGoroutine = 100

	done := make(chan struct{})

	// Concurrently register PIDs
	for i := range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < pidsPerGoroutine; j++ {
				r.register(i*pidsPerGoroutine + j)
			}
		}()
	}

	for range goroutines {
		<-done
	}

	// All PIDs should be registered
	for i := range goroutines {
		for j := 0; j < pidsPerGoroutine; j++ {
			assert.True(t, r.IsActive(i*pidsPerGoroutine+j))
		}
	}

	// Concurrently unregister PIDs
	for i := range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < pidsPerGoroutine; j++ {
				r.unregister(i*pidsPerGoroutine + j)
			}
		}()
	}

	for range goroutines {
		<-done
	}

	// All PIDs should be unregistered
	for i := range goroutines {
		for j := 0; j < pidsPerGoroutine; j++ {
			assert.False(t, r.IsActive(i*pidsPerGoroutine+j))
		}
	}
}

func TestTracker_IsActive(t *testing.T) {
	r := newTestRegistry(t)
	tracker := Tracker()

	// PID not registered
	assert.False(t, tracker.IsActive(42))

	// Register via internal helper (same path as executor methods)
	r.register(42)
	assert.True(t, tracker.IsActive(42))

	r.unregister(42)
	assert.False(t, tracker.IsActive(42))
}

func TestGlobalRegistry_Output_RegistersPID(t *testing.T) {
	newTestRegistry(t)

	ex := NewExecutor("", "echo", []string{"hello"}, []string{})

	output, err := ex.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(output), "hello")

	// PID should be unregistered after Output returns.
}

func TestGlobalRegistry_Output_FailedStart(t *testing.T) {
	newTestRegistry(t)

	// Command that doesn't exist — Start() should fail.
	ex := NewExecutor("", "/nonexistent/binary", []string{}, []string{})
	_, err := ex.Output()
	assert.Error(t, err)
}

func TestGlobalRegistry_RunAndLogLines_RegistersPID(t *testing.T) {
	newTestRegistry(t)

	logger := log.NewLogger()
	logger.SetLevel(log.LevelInfo)

	ex := NewExecutor("", "echo", []string{"test-output"}, []string{}).
		WithLogger(logger)

	usage, err := ex.RunAndLogLines(context.Background(), map[string]string{})
	assert.NoError(t, err)
	assert.NotNil(t, usage)
}

func TestGlobalRegistry_RunAndLogLines_FailedStart(t *testing.T) {
	newTestRegistry(t)

	logger := log.NewLogger()

	ex := NewExecutor("", "/nonexistent/binary", []string{}, []string{}).
		WithLogger(logger)

	_, err := ex.RunAndLogLines(context.Background(), map[string]string{})
	assert.Error(t, err)
}
