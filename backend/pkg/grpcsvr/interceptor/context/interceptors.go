package context

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"google.golang.org/grpc"

	"github.com/wangweihong/gotoolbox/pkg/tracectx"
)

// UnaryServerInterceptor returns a new unary server interceptor for trace.
func UnaryServerInterceptor(skipperFunc ...skipper.SkipperFunc) grpc.UnaryServerInterceptor {
	name := "context"

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		log.F(ctx).Debugf("Interceptor %s Enter", name)
		defer log.F(ctx).Debugf("Interceptor %s Finish", name)

		if skipper.Skip(info.FullMethod, skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for %s", name, info.FullMethod)

			resp, err := handler(ctx, req)
			return resp, errors.WithStack(err)
		}

		traceID := tracectx.FromTraceIDContext(ctx)
		// don't inject too much field in global context
		fields := make(map[string]any)
		fields[string(log.KeyRequestID)] = traceID
		ctx = log.WithFields(ctx, fields)

		// 调用下一个拦截器或最终的RPC处理程序
		resp, err := handler(ctx, req)
		return resp, errors.WithStack(err)
	}
}

// StreamServerInterceptor returns a new streaming server interceptor for trace.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		// TODO: how to trace stream request?
		return handler(srv, stream)
	}
}
