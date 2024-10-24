package unilogger_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/unilogger"

	"github.com/stretchr/testify/assert"
)

func Test_Logger(t *testing.T) {
	t.Parallel()

	defaultLogFn := func(logger *unilogger.Logger) {
		logger.Trace("stub msg", slog.String("stub_arg", "arg"))
		logger.Debug("stub msg", slog.String("stub_arg", "arg"))
		logger.Info("stub msg", slog.String("stub_arg", "arg"))
		logger.Warn("stub msg", slog.String("stub_arg", "arg"))
		logger.Error("stub msg", slog.String("stub_arg", "arg"))
		//test fatal
		logger.Log(context.Background(), unilogger.LevelFatal.Level(), "stub msg", slog.String("stub_arg", "arg"))
	}

	logfFn := func(logger *unilogger.Logger) {
		logger.Tracef("stub msg: %s", "arg")
		logger.Debugf("stub msg: %s", "arg")
		logger.Infof("stub msg: %s", "arg")
		logger.Warnf("stub msg: %s", "arg")
		logger.Errorf("stub msg: %s", "arg")
		//test fatal
		logger.Logf(context.Background(), unilogger.LevelFatal, "stub msg: %s", "arg")
	}

	type meta struct {
		name    string
		enabled bool
	}

	type fields struct {
		logfn          func(logger *unilogger.Logger)
		mutateLoggerfn func(logger *unilogger.Logger) *unilogger.Logger
	}

	type args struct {
		addSource bool
		level     unilogger.Level
	}

	type wants struct {
		shouldContains    []string
		shouldNotContains []string
	}

	tests := []struct {
		meta   meta
		fields fields
		args   args
		wants  wants
	}{
		{
			meta: meta{
				name:    "logger default options is level info and add source false",
				enabled: true,
			},
			fields: fields{
				logfn:          defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger { return logger },
			},
			args: args{},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg","stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					logger.SetLevel(unilogger.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"debug","msg":"stub msg","source":"unilogger/logger_test.go:20","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"info","msg":"stub msg","source":"unilogger/logger_test.go:21","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg","source":"unilogger/logger_test.go:22","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg","stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg","source":"unilogger/logger_test.go:25","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "*f functions logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: logfFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					logger.SetLevel(unilogger.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"debug","msg":"stub msg: arg","source":"unilogger/logger_test.go:30","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"info","msg":"stub msg: arg","source":"unilogger/logger_test.go:31","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg: arg","source":"unilogger/logger_test.go:32","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg: arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg: arg","source":"unilogger/logger_test.go:35","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "logger with name should have field logger",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					return logger.Named("first")
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","logger":"first","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","logger":"first","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","logger":"first","msg":"stub msg","stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","logger":"first","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "logger names should separate by dot",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					logger = logger.Named("first")
					logger = logger.Named("second")
					return logger.Named("third")
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "with group should wrap args",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					return logger.WithGroup("module")
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","msg":"stub msg","module":{"stub_arg":"arg"},"time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg","module":{"stub_arg":"arg"},"time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg","module":{"stub_arg":"arg"},"stacktrace":`,
					`{"level":"fatal","msg":"stub msg","module":{"stub_arg":"arg"},"time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "raw json arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					return logger.With(unilogger.RawJSON("stub log", `{"stub arg":{"nested arg":"some"}}`))
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "raw yaml arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					return logger.With(unilogger.RawYAML("stub log", `
stubArg:
  nestedArg: some`))
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"info","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"warn","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
					`{"level":"error","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
		{
			meta: meta{
				name:    "default logger level change must affect logger which set default",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *unilogger.Logger) *unilogger.Logger {
					unilogger.SetDefault(logger)
					unilogger.SetDefaultLevel(unilogger.LevelError)
					return logger
				},
			},
			args: args{
				addSource: false,
				level:     unilogger.LevelInfo,
			},
			wants: wants{
				shouldContains: []string{
					`{"level":"error","msg":"stub msg","stub_arg":"arg","stacktrace":`,
					`{"level":"fatal","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}`,
				},
				shouldNotContains: []string{
					`"level":"info"`,
					`"level":"warn"`,
					`"level":"debug"`,
					`"level":"trace"`,
				},
			},
		},
	}

	for _, tt := range tests {
		if !tt.meta.enabled {
			continue
		}

		t.Run(tt.meta.name, func(t *testing.T) {
			buf := bytes.NewBuffer([]byte{})

			logger := unilogger.NewLogger(unilogger.Options{
				Level:  tt.args.level.Level(),
				Output: buf,
				TimeFunc: func(_ time.Time) time.Time {
					t, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
					if err != nil {
						panic(err)
					}

					return t
				},
			})

			logger = tt.fields.mutateLoggerfn(logger)

			tt.fields.logfn(logger)

			for _, v := range tt.wants.shouldContains {
				assert.Contains(t, buf.String(), v)
			}

			for _, v := range tt.wants.shouldNotContains {
				assert.NotContains(t, buf.String(), v)
			}
		})
	}
}
