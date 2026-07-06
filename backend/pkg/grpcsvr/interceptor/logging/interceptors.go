package logging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"google.golang.org/grpc/peer"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/netutil"

	"google.golang.org/grpc"
)

var (
	DisableCopy             bool
	MaxRequestLoggerLength  = 4096
	MaxResponseLoggerLength = 4096
)

// UnaryServerInterceptor returns a new unary server interceptor for trace.
func UnaryServerInterceptor(skipperFunc ...skipper.SkipperFunc) grpc.UnaryServerInterceptor {
	name := "logging"

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		log.F(ctx).Debugf("Interceptor %s Enter", name)
		defer log.F(ctx).Debugf("Interceptor %s Finish", name)

		if skipper.Skip(info.FullMethod, skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for %s", name, info.FullMethod)

			resp, err := handler(ctx, req)
			return resp, errors.WithStack(err)
		}

		// 调用下一个拦截器或最终的RPC处理程序
		start := time.Now()
		fields := make(map[string]any)
		clientIP := getClientIPFromContext(ctx)
		// log会根据key的排序来依次打印，调整key的命名以达到控制输出顺序
		fields["host_pid"] = os.Getpid()
		fields["host_ip"] = netutil.GetIPAddrNotError(true)
		fields["req_time_begin"] = start.Format("2006-01-02 15:04:05.000000")
		fields["req_client_ip"] = clientIP
		fields["req_method"] = info.FullMethod
		fields["req_param"] = req

		resp, err := handler(ctx, req)

		end := time.Now()
		Latency := time.Since(start)
		if Latency > time.Minute {
			// Truncate in a golang < 1.8 safe way
			Latency -= Latency % time.Second
		}

		fields["req_latency_ms"] = Latency
		fields["req_time_end"] = end.Format("2006-01-02 15:04:05.000000")
		fields["resp_err"] = errors.Message(err)
		if !DisableCopy {
			fields["resp_body"] = resp
		}

		simpleCallInfo := fmt.Sprintf("[%s] %v %s", clientIP, Latency, info.FullMethod)
		log.F(ctx).Info(simpleCallInfo, log.Every("call-detail", fields))
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

// getClientIPFromContext 从上下文中获取客户端的 IP 地址.
func getClientIPFromContext(ctx context.Context) string {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	addr := peer.Addr.String()
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return "unknown"
	}

	return addr[:idx]
}
