package logging

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"google.golang.org/grpc"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

// UnaryClientInterceptor returns a new unary client interceptor for logging.
func UnaryClientInterceptor(skipperFunc ...skipper.SkipperFunc) grpc.UnaryClientInterceptor {
	name := "logging"

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		log.F(ctx).Debugf("Interceptor %s Enter", name)
		defer log.F(ctx).Debugf("Interceptor %s Finish", name)

		if skipper.Skip(method, skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for method %s", name, method)
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		log.F(ctx).Debug("request param", log.Every("req", req), log.String("method", method))
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return errors.WithStack(err)
		}
		log.F(ctx).Debug("response data", log.Every("out", reply))
		return nil
	}
}

func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, nil
	}
}
