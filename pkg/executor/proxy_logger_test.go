package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/require"

	json "github.com/flant/shell-operator/pkg/utils/json"
)

func TestProxyLoggerWriteBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		proxy    bool
		input    string
		expected []map[string]any
	}{
		{
			name:  "mixed JSON and text",
			proxy: true,
			input: "{\"level\":\"error\",\"msg\":\"quoted \\\"{value}\\\"\",\"value\":\"Привет\"}\r\n" +
				"plain text\r\n" + "{\r\n\"a\":\"b\",\r\n\"c\":\"d\"\r\n}",
			expected: []map[string]any{
				{"level": "error", "msg": `quoted "{value}"`, "hook_value": "Привет", "output": "stdout", "proxyJsonLog": true},
				{"level": "info", "msg": "plain text", "output": "stdout"},
				{"level": "info", "msg": "hook result", "hook_a": "b", "hook_c": "d", "output": "stdout", "proxyJsonLog": true},
			},
		},
		{
			name:  "plain text without JSON proxy",
			input: "first line\r\n\nПривет\nlast line",
			expected: []map[string]any{
				{"level": "info", "msg": "first line", "output": "stdout"},
				{"level": "info", "msg": "Привет", "output": "stdout"},
				{"level": "info", "msg": "last line", "output": "stdout"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("whole", func(t *testing.T) {
				require.Equal(t, tc.expected, proxyLoggerRecords(t, tc.proxy, tc.input))
			})
			for split := 1; split < len(tc.input); split++ {
				t.Run(fmt.Sprintf("split_%d", split), func(t *testing.T) {
					require.Equal(t, tc.expected, proxyLoggerRecords(t, tc.proxy, tc.input[:split], tc.input[split:]))
				})
			}
			t.Run("bytewise", func(t *testing.T) {
				chunks := make([]string, len(tc.input))
				for i := range chunks {
					chunks[i] = tc.input[i : i+1]
				}
				require.Equal(t, tc.expected, proxyLoggerRecords(t, tc.proxy, chunks...))
			})
		})
	}
}

func TestProxyLoggerFlushIncompleteJSON(t *testing.T) {
	require.Equal(t, []map[string]any{
		{"level": "info", "msg": `{"unfinished":`, "output": "stdout"},
	}, proxyLoggerRecords(t, true, `{"unfin`, `ished":`))
}

func TestProxyLoggerMalformedJSON(t *testing.T) {
	require.Equal(t, []map[string]any{
		{"level": "info", "msg": `{"value": broken}`, "output": "stdout"},
		{"level": "info", "msg": "hook result", "hook_value": "next", "output": "stdout", "proxyJsonLog": true},
	}, proxyLoggerRecords(t, true, "{\"value\": broken}\n{\"value\":\"next\"}\n"))
}

func TestProxyLoggerMultilineArray(t *testing.T) {
	input := "[\n\n\"first\",\n\"second\"\n]\n"
	require.Equal(t, []map[string]any{
		{"level": "info", "msg": input, "output": "stdout"},
	}, proxyLoggerRecords(t, true, input[:4], input[4:]))
}

func TestProxyLoggerLongRecords(t *testing.T) {
	t.Run("plain text is truncated across writes", func(t *testing.T) {
		line := "result " + strings.Repeat("x", 20000)
		records := proxyLoggerRecords(t, false, line[:4096], line[4096:]+"\nafter\n")
		require.Equal(t, []map[string]any{
			{"level": "info", "msg": line[:10000] + ":truncated", "output": "stdout"},
			{"level": "info", "msg": "after", "output": "stdout"},
		}, records)
	})

	t.Run("JSON fields are truncated across writes", func(t *testing.T) {
		input := `{"payload":"` + strings.Repeat("x", 70000) + `"}`
		records := proxyLoggerRecords(t, true, input[:32768], input[32768:]+"\n")
		require.Equal(t, []map[string]any{
			{
				"level": "info", "msg": "hook result", "output": "stdout", "proxyJsonLog": true,
				"hook": map[string]any{"truncated": input[:10000] + ":truncated"},
			},
		}, records)
	})

	t.Run("non-object JSON remains raw text", func(t *testing.T) {
		input := `"` + strings.Repeat("x", 70000) + "\"\n"
		require.Equal(t, []map[string]any{
			{"level": "info", "msg": input, "output": "stdout"},
		}, proxyLoggerRecords(t, true, input[:32768], input[32768:]))
	})
}

func TestRunAndLogLinesFlushesBothStreams(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed_%t", failed), func(t *testing.T) {
			logger, output := newProxyLoggerTestLogger()
			runner := &fakeRunner{stdout: `{"value":"out"}`, stderr: "stderr tail"}
			if failed {
				runner.err = errors.New("hook failed")
			}
			ex := NewExecutor("", "hook", nil, nil).
				WithRunner(runner).
				WithLogProxyHookJSON(true).
				WithLogger(logger)

			_, err := ex.RunAndLogLines(context.Background(), nil)
			if failed {
				var stderrErr *StderrError
				require.ErrorAs(t, err, &stderrErr)
				require.Equal(t, runner.stderr, stderrErr.Message)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, []map[string]any{
				{"level": "info", "msg": "hook result", "hook_value": "out", "output": "stdout", "proxyJsonLog": true},
				{"level": "info", "msg": "stderr tail", "output": "stderr"},
			}, decodeProxyLoggerRecords(t, output.String()))
		})
	}
}

func proxyLoggerRecords(t *testing.T, proxy bool, chunks ...string) []map[string]any {
	t.Helper()
	logger, output := newProxyLoggerTestLogger()
	pl := &proxyLogger{
		ctx:              context.Background(),
		logProxyHookJSON: proxy,
		proxyJsonLogKey:  "proxyJsonLog",
		logger:           logger.With("output", "stdout"),
	}
	for _, chunk := range chunks {
		n, err := pl.Write([]byte(chunk))
		require.NoError(t, err)
		require.Equal(t, len(chunk), n)
	}
	pl.flush()
	result := output.String()
	pl.flush()
	require.Equal(t, result, output.String(), "flushing again must not duplicate records")
	return decodeProxyLoggerRecords(t, result)
}

func newProxyLoggerTestLogger() (*log.Logger, *bytes.Buffer) {
	logger := log.NewLogger(log.WithTimeFunc(func(time.Time) time.Time {
		return time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)
	}))
	logger.SetLevel(log.LevelInfo)
	output := new(bytes.Buffer)
	logger.SetOutput(output)
	return logger, output
}

func decodeProxyLoggerRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		require.Equal(t, "2006-01-02T15:04:05Z", record["time"])
		delete(record, "time")
		records = append(records, record)
	}
	return records
}
