package hook

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/hook/config"
	. "github.com/onsi/gomega"
	"golang.org/x/time/rate"

	. "github.com/flant/shell-operator/pkg/hook/types"
)

func Test_Hook_SafeName(t *testing.T) {
	g := NewWithT(t)

	WorkingDir := "/hooks"
	hookPath := "/hooks/002-cool-hooks/monitor-namespaces.py"

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		t.Error(err)
	}

	h := NewHook(hookName, hookPath)

	g.Expect(h.SafeName()).To(Equal("002-cool-hooks-monitor-namespaces-py"))
}

func Test_CreateLimiter(t *testing.T) {
	const (
		defaultBurst = 1
		defaultLimit = rate.Inf
	)

	g := NewWithT(t)

	cases := []struct {
		burst    int
		limit    rate.Limit
		title    string
		settings *Settings
	}{
		{
			title:    "Nil run settings: should return limiter with defaults",
			limit:    defaultLimit,
			burst:    defaultBurst,
			settings: nil,
		},

		{
			title:    "Empty settings: should return limiter with defaults",
			limit:    defaultLimit,
			burst:    defaultBurst,
			settings: &Settings{},
		},

		{
			title: "Burst is zero limit is none zero: should return limiter with zero burst and converted interval",
			limit: rate.Limit(1 / 20.0),
			burst: 0,
			settings: &Settings{
				ExecutionMinInterval: 20 * time.Second,
			},
		},

		{
			title: "Burst is none zero limit is zero: should return limiter with default limiter and passed burst",
			limit: defaultLimit,
			burst: 3,
			settings: &Settings{
				ExecutionBurst: 3,
			},
		},

		{
			title: "All settings passed: should run limiter with all passed burst and converted interval",
			limit: rate.Limit(1.0 / 30),
			burst: 3,
			settings: &Settings{
				ExecutionBurst:       3,
				ExecutionMinInterval: 30 * time.Second,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			cfg := &config.HookConfig{
				Settings: c.settings,
			}

			l := CreateRateLimiter(cfg)

			g.Expect(l.Burst(), c.burst)
			g.Expect(l.Limit(), c.limit)
		})
	}
}

func Test_Hook_WithConfig(t *testing.T) {
	g := NewWithT(t)

	var hook *Hook
	var err error

	tests := []struct {
		name     string
		jsonData string
		fn       func()
	}{
		{
			"simple",
			`{"onStartup": 10}`,
			func() {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(hook.Config).ToNot(BeNil())
				g.Expect(hook.Config.Bindings()).To(Equal([]BindingType{OnStartup}))
				g.Expect(hook.Config.OnStartup).ToNot(BeNil())
				g.Expect(hook.Config.OnStartup.Order).To(Equal(10.0))
			},
		},
		{
			"with validation error",
			`{"configVersion":"v1", "onStartup": "10"}`,
			func() {
				g.Expect(err).Should(HaveOccurred())
				g.Expect(err.Error()).To(MatchRegexp("onStartup .*must be of type integer: \"string\""))

				//t.Logf("expected validation error was: %v", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hook = NewHook("hook-sh", "/hooks/hook.sh")
			_, err = hook.LoadConfig([]byte(test.jsonData))
			test.fn()
		})
	}
}
