package tracing

//
//// UnaryClientInterceptor returns a new unary client interceptor for logging.
//func UnaryClientInterceptor(skipperFunc ...skipper.SkipperFunc) grpc.UnaryClientInterceptor {
//	tracer := otel.GetTracerProvider().Tracer("tracer")
//
// 	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker
// grpc.UnaryInvoker, opts ...grpc.CallOption) error {
//		name,attr,_:= mtrace.
//		return nil
//	}
//}
//
//func StreamClientInterceptor() grpc.StreamClientInterceptor {
// 	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer,
// opts ...grpc.CallOption) (grpc.ClientStream, error) {
//		return nil, nil
//	}
//}
