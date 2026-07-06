package interceptorcli

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

func LoggingInterceptor(name string, skipperFunc ...skipper.SkipperFunc) httpcli.Interceptor {
	return httpcli.NewInterceptor(name, func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client,
		invoker httpcli.Invoker, opts ...httpcli.CallOption) (*httpcli.HttpResponse, error) {
		if skipper.Skip(req.GetPath(), skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for rawrurl %s", name, req.GetPath())

			return invoker(ctx, req, arg, reply, cc, opts...)
		}
		start := time.Now()
		fields := make(map[string]any)
		fields["req_time_begin"] = start.Format("2006-01-02 15:04:05.000000")
		fields["req_raw_url"] = req.GetPath()
		fields["method"] = req.GetMethod()

		rawResp, err := invoker(ctx, req, arg, reply, cc, opts...)

		end := time.Now()
		Latency := time.Since(start)
		if Latency > time.Minute {
			// Truncate in a golang < 1.8 safe way
			Latency -= Latency % time.Second
		}
		fields["req_latency_ms"] = Latency
		fields["req_time_end"] = end.Format("2006-01-02 15:04:05.000000")

		reqURL := req.GetFullRequestAddress()
		reqAddr := req.GetEndpoint()
		method := req.GetMethod()
		var statusCode int
		if rawResp != nil {
			statusCode = rawResp.GetStatusCode()
			fields["resp_status"] = rawResp.GetStatusCode()
			fields["resp_length"] = len(rawResp.GetBody())
			fields["resp_header"] = rawResp.GetHeaders()
			fields["req_url"] = reqURL
			fields["req_media_type"] = rawResp.GetHeader("Content-Type")
			fields["req_addr"] = reqAddr
		}
		simpleCallInfo := fmt.Sprintf(
			"%3d - [%s] %v %s  %s",
			statusCode,
			reqAddr,
			Latency,
			method,
			reqURL,
		)
		log.L(ctx).Fields(fields).Info(simpleCallInfo)

		if err != nil {
			return rawResp, err
		}
		return rawResp, nil
	})
}
