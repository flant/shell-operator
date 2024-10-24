package unilogger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	logContext "github.com/flant/shell-operator/pkg/unilogger/context"
)

// Extends default slog with new log levels
type WrappedLogger struct {
	*slog.Logger
	opts *slog.HandlerOptions
}

func NewSlogLogger(w io.Writer, opts *slog.HandlerOptions) *WrappedLogger {
	return &WrappedLogger{
		Logger: slog.New(slog.NewJSONHandler(w, opts)),
		opts:   opts,
	}
}

var _ slog.Handler = (*SlogHandler)(nil)

type SlogHandler struct {
	slog.Handler

	w io.Writer
	b *bytes.Buffer
	m *sync.Mutex

	timeFn func(t time.Time) time.Time
}

func NewSlogHandler(handler slog.Handler) *SlogHandler {
	return &SlogHandler{
		Handler: handler,
	}
}

func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.m.Lock()

	defer func() {
		h.b.Reset()
		h.m.Unlock()
	}()

	var (
		out      []byte
		tracePtr *string
	)

	isCustom := logContext.GetCustomKeyContext(ctx)
	if isCustom {
		// r.PC, _, _, _ = runtime.Caller(4)
		var pc uintptr
		var pcs [1]uintptr
		// skip [runtime.Callers, this function, this function's caller]
		runtime.Callers(5, pcs[:])
		pc = pcs[0]
		r.PC = pc

		tracePtr = logContext.GetStackTraceContext(ctx)
	}

	if err := h.Handler.Handle(ctx, r); err != nil {
		return err
	}

	attrs := map[string]any{}
	if err := json.Unmarshal(h.b.Bytes(), &attrs); err != nil {
		return err
	}

	// HEAD start
	var headLogFields []string
	lvl := fmt.Sprintf(`"level":"%s"`, strings.ToLower(Level(r.Level).String()))
	time := fmt.Sprintf(`"time":"%s"`, h.timeFn(r.Time).Format(time.RFC3339))
	msg := fmt.Sprintf(`"msg":"%s"`, r.Message)

	headLogFields = append(headLogFields, lvl)

	// if logger was named
	loggerName, ok := attrs["logger"]
	if ok {
		name := fmt.Sprintf(`"logger":"%s"`, loggerName)
		headLogFields = append(headLogFields, name)

		delete(attrs, "logger")
	}

	headLogFields = append(headLogFields, msg)

	// FOOT start
	var footLogFields []string

	// if logger was named
	if tracePtr != nil {
		trace := fmt.Sprintf(`"stacktrace":"%s"`, *tracePtr)
		footLogFields = append(footLogFields, trace)

		delete(attrs, "source")
	}

	footLogFields = append(footLogFields, time)

	fieldSource, ok := attrs["source"]
	if ok {
		src := fmt.Sprintf(`"source":"%s"`, fieldSource)
		headLogFields = append(headLogFields, src)

		delete(attrs, "source")
	}

	b, err := json.Marshal(attrs)
	if err != nil {
		return err
	}

	rawHeadLogFields := strings.Join(headLogFields, ",")
	rawfootLogFields := strings.Join(footLogFields, ",")

	out = append(out, '{')

	out = append(out, []byte(rawHeadLogFields)...)

	out = append(out, ',')

	if len(attrs) > 0 {
		out = append(out, b[1:len(b)-1]...)
		out = append(out, ',')
	}

	out = append(out, []byte(rawfootLogFields)...)

	out = append(out, '}')

	h.w.Write(append(out, "\n"...))

	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) < 1 {
		return h
	}

	h2 := *h
	h2.Handler = h.Handler.WithAttrs(attrs)

	return &h2
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.Handler = h.Handler.WithGroup(name)

	return &h2
}

func NewHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *SlogHandler {
	b := new(bytes.Buffer)

	return &SlogHandler{
		Handler: slog.NewJSONHandler(b, opts),
		b:       b,
		m:       &sync.Mutex{},
		w:       out,
		timeFn:  timeFn,
	}
}
