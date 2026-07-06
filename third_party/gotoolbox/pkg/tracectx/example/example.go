package main

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/tracectx"
)

func main() {
	defer log.Flush()

	// example1
	ctx := tracectx.WithTraceIDContext(context.Background())
	traceID := tracectx.FromTraceIDContext(ctx)
	ctx = log.WithFieldPair(ctx, "traceID", traceID)

	log.F(ctx).Info("aaa")
	log.F(ctx).Info("bbb")

	ctx = log.F(ctx).WithContext(ctx)
	log.FromContext(ctx).Info("cccc")

	// example2
	ctx = log.WithFieldPair(context.Background(), "X-Request-ID", tracectx.NewTraceID())
	ctx = log.F(ctx).WithContext(ctx)
	log.FromContext(ctx).Info("example2")
}
