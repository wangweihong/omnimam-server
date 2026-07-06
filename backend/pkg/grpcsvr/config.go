package grpcsvr

import (
	"github.com/wangweihong/gotoolbox/pkg/debug"
	"github.com/wangweihong/gotoolbox/pkg/tls/grpctls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor"

	"github.com/wangweihong/gotoolbox/pkg/log"
	//"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc".
)

// GRPCConfig defines  configuration for the grpc server.
type GRPCConfig struct {
	TlsEnable          bool
	Addr               string
	UnixSocket         string
	MaxMsgSize         int
	ServerCert         tls.CertData
	ClientCA           string
	Version            bool
	Reflect            bool
	Debug              bool
	UnaryInterceptors  []string
	StreamInterceptors []string
	RuntimeDebug       *debug.RuntimeDebugInfo
}

// NewConfig returns a Config struct with the default values.
func NewConfig() *GRPCConfig {
	return &GRPCConfig{
		Version: true,
		Reflect: false,
		Debug:   false,
		UnaryInterceptors: []string{
			interceptor.InterceptorNameRequestID,
			interceptor.InterceptorNameContext,
			interceptor.InterceptorNameLogger,
			interceptor.InterceptorNameRecovery,
		},
		RuntimeDebug: &debug.RuntimeDebugInfo{
			Enable:    false,
			OutputDir: "",
		},
	}
}

type CompletedGRPCConfig struct {
	*GRPCConfig
}

// Complete fills in any fields not set that are required to have valid data and can be derived from other fields.
func (c *GRPCConfig) Complete() *CompletedGRPCConfig {
	return &CompletedGRPCConfig{c}
}

// New create a grpc Server instance.
func (c *CompletedGRPCConfig) New() (*GRPCServer, error) {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(c.MaxMsgSize),
	}

	if c.TlsEnable {
		var creds credentials.TransportCredentials
		var err error
		if c.ClientCA == "" {
			log.Info("gRPC service run with TLS")

			creds, err = grpctls.NewTlsServerCredentials([]byte(c.ServerCert.Cert), []byte(c.ServerCert.Key))
		} else {
			log.Info("gRPC service run with mTLS")

			creds, err = grpctls.NewMutualTlsServerCredentials([]byte(c.ClientCA), []byte(c.ServerCert.Cert), []byte(c.ServerCert.Key))
		}

		if err != nil {
			log.Fatalf("Failed to generate credentials %s", err.Error())
		}

		opts = append(opts, grpc.Creds(creds))
	}

	opts = installInterceptors(c.UnaryInterceptors, opts)
	// opts = append(opts, grpc.ChainStreamInterceptor(streamUnaryInterceptor...))

	gRPCServer := &GRPCServer{
		Server:             grpc.NewServer(opts...),
		UnixSocket:         c.UnixSocket,
		Address:            c.Addr,
		Version:            c.Version,
		Reflect:            c.Reflect,
		Debug:              c.Debug,
		UnaryInterceptors:  c.UnaryInterceptors,
		StreamInterceptors: c.StreamInterceptors,
		runtimeDebug:       c.RuntimeDebug,
	}

	initGenericGRPCServer(gRPCServer)
	return gRPCServer, nil
}
