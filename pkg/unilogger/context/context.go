package slog

import (
	"context"
)

type ctxKey string

const customKey ctxKey = "custom_key"
const stackTrace ctxKey = "stack_trace"

func SetCustomKeyContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, customKey, true)
}

func GetCustomKeyContext(ctx context.Context) bool {
	has, ok := ctx.Value(customKey).(bool)
	if !ok {
		return false
	}

	return has
}

func SetStackTraceContext(ctx context.Context, trace string) context.Context {
	return context.WithValue(ctx, stackTrace, trace)
}

func GetStackTraceContext(ctx context.Context) *string {
	trace, ok := ctx.Value(stackTrace).(string)
	if !ok {
		return nil
	}

	return &trace
}
