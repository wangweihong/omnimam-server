package tracectx

import (
	"context"
)

type (
	TraceIDKey struct{} // store traceID in context
)

func NewTraceIDContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey{}, traceID)
}

// FromTraceIDContext get trace id from context.
func FromTraceIDContext(ctx context.Context) string {
	if v := ctx.Value(TraceIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func WithTraceIDContext(ctx context.Context) context.Context {
	if v := ctx.Value(TraceIDKey{}); v == nil {
		traceID := NewTraceID()
		return context.WithValue(ctx, TraceIDKey{}, traceID)
	}
	return ctx
}
