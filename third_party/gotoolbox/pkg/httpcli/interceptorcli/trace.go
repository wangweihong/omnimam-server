package interceptorcli

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/skipper"
	"github.com/wangweihong/gotoolbox/pkg/tracectx"
)

func TraceInterceptor(name string, skipperFunc ...skipper.SkipperFunc) httpcli.Interceptor {
	return httpcli.NewInterceptor(name, func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client,
		invoker httpcli.Invoker, opts ...httpcli.CallOption,
	) (*httpcli.HttpResponse, error) {
		if skipper.Skip(req.GetPath(), skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for rawrurl %s", name, req.GetPath())
			return invoker(ctx, req, arg, reply, cc, opts...)
		}

		traceID := tracectx.NewTraceID()
		req.Builder().AddHeaderParam(tracectx.XRequestIDKey, traceID).Build()

		ctx = context.WithValue(ctx, log.KeyRequestID, traceID)
		rawResp, err := invoker(ctx, req, arg, reply, cc, opts...)
		return rawResp, err
	})
}
