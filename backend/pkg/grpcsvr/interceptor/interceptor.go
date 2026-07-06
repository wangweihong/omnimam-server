package interceptor

import (
	"google.golang.org/grpc"

	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor/context"
	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor/logging"
	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor/recovery"
	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor/requestid"
)

const (
	InterceptorNameContext   = "context"
	InterceptorNameRequestID = "requestid"
	InterceptorNameRecovery  = "recovery"
	InterceptorNameLogger    = "logger"
)

var (
	UnaryServerInterceptorList  = defaultUnaryServerInterceptorList()
	UnaryServerInterceptorNames = defaultInterceptorListNames()
)

func defaultUnaryServerInterceptorList() map[string]grpc.UnaryServerInterceptor {
	return map[string]grpc.UnaryServerInterceptor{
		InterceptorNameContext:   context.UnaryServerInterceptor(),
		InterceptorNameRequestID: requestid.UnaryServerInterceptor(),
		InterceptorNameRecovery: recovery.UnaryServerInterceptor(
			recovery.WithRecoveryHandlerContext(recovery.CustomPanicHandler),
		),
		InterceptorNameLogger: logging.UnaryServerInterceptor(),
	}
}

func defaultInterceptorListNames() []string {
	names := make([]string, 0, len(defaultUnaryServerInterceptorList()))
	for name := range defaultUnaryServerInterceptorList() {
		names = append(names, name)
	}
	return names
}
