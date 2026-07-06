package mtls

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/wangweihong/omnimam/backend/internal/pkg/code"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// UnaryServerInterceptor returns a new unary server interceptor for mtls verify.
func UnaryServerInterceptor(skipperFunc ...skipper.SkipperFunc) grpc.UnaryServerInterceptor {
	// name := "mtls"

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skipper.Skip(info.FullMethod, skipperFunc...) {
			resp, err := handler(ctx, req)
			return resp, errors.WithStack(err)
		}

		peer, ok := peerFromContext(ctx)
		if !ok {
			log.F(ctx).Error("failed to get client peer information")
			return nil, errors.WithCode(code.ErrGRPCClientCertificateError, "failed to get client peer information")
		}

		if peer == nil || peer.AuthInfo == nil {
			log.F(ctx).Error("client is not authenticated")
			return nil, errors.WithCode(code.ErrGRPCClientCertificateError, "client is not authenticated")
		}

		// 获取客户端证书信息
		tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
		if !ok {
			log.F(ctx).Error("failed to get TLSInfo from client AuthInfo")
			return nil, errors.WithCode(
				code.ErrGRPCClientCertificateError,
				"failed to get TLSInfo from client AuthInfo",
			)
		}

		// 获取客户端证书
		certificates := tlsInfo.State.PeerCertificates
		if len(certificates) == 0 {
			log.F(ctx).Error("client certificate is missing")
			return nil, errors.WithCode(code.ErrGRPCClientCertificateError, "client certificate is missing")
		}

		// 验证客户端证书的主体信息
		clientCert := certificates[0]
		if clientCert.Subject.CommonName != "client.example.com" {
			log.F(ctx).Error("invalid client certificate subject")
			return nil, errors.WithCode(code.ErrGRPCClientCertificateError, "invalid client certificate subject")
		}

		resp, err := handler(ctx, req)
		return resp, errors.WithStack(err)
	}
}

// StreamServerInterceptor returns a new streaming server interceptor for trace.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		peer, ok := peerFromContext(stream.Context())
		if !ok {
			log.Error("failed to get client peer information")
			return errors.WithCode(code.ErrGRPCClientCertificateError, "failed to get client peer information")
		}

		if peer == nil || peer.AuthInfo == nil {
			log.Error("client is not authenticated")
			return errors.WithCode(code.ErrGRPCClientCertificateError, "client is not authenticated")
		}

		// 获取客户端证书信息
		tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
		if !ok {
			log.Error("failed to get TLSInfo from client AuthInfo")
			return errors.WithCode(code.ErrGRPCClientCertificateError, "failed to get TLSInfo from client AuthInfo")
		}
		// 获取客户端证书
		certificates := tlsInfo.State.PeerCertificates
		if len(certificates) == 0 {
			log.Error("client certificate is missing")
			return errors.WithCode(code.ErrGRPCClientCertificateError, "client certificate is missing")
		}

		// 验证客户端证书的主体信息
		clientCert := certificates[0]
		if clientCert.Subject.CommonName != "client.example.com" {
			log.Error("invalid client certificate subject")
			return errors.WithCode(code.ErrGRPCClientCertificateError, "invalid client certificate subject")
		}

		return handler(srv, stream)
	}
}

// 从 gRPC context 中获取客户端信息.
func peerFromContext(ctx context.Context) (*peer.Peer, bool) {
	p, ok := peer.FromContext(ctx)
	return p, ok
}
