package grpcsvr

import (
	"net"
	"os"
	"path/filepath"

	"github.com/wangweihong/gotoolbox/pkg/debug"

	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor"

	"google.golang.org/grpc/reflection"

	"github.com/wangweihong/omnimam/backend/pkg/grpcproto/service/debugservice"
	"github.com/wangweihong/omnimam/backend/pkg/grpcproto/service/versionservice"

	"golang.org/x/sync/errgroup"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"google.golang.org/grpc"
)

type GRPCServer struct {
	*grpc.Server
	UnixSocket string
	Address    string
	// install services
	Version bool
	Reflect bool
	Debug   bool
	// install interceptors
	UnaryInterceptors  []string
	StreamInterceptors []string

	runtimeDebug *debug.RuntimeDebugInfo
}

func (s *GRPCServer) Run() {
	var eg errgroup.Group

	if s.Address != "" {
		eg.Go(func() error {
			log.Infof("start gRPC server at tcp://%s", s.Address)
			listen, err := net.Listen("tcp", s.Address)
			if err != nil {
				log.Fatalf("failed to listen on tcp://%s : %v", s.Address, err)
			}

			log.Infof("gRPC Listen at %v", listen.Addr())
			if err := s.Serve(listen); err != nil {
				log.Fatalf("failed to serve grpc server tcp://%v : %v", s.Address, err)
			}
			return nil
		})
	}

	if s.UnixSocket != "" {
		eg.Go(func() error {
			// If s.UnixSocket file exist before `Listen`, net Listen will fail with `bind: address already in use`
			if err := os.Remove(s.UnixSocket); err != nil && !os.IsNotExist(err) {
				log.Fatalf("unix socket file %v already in use, remove fail:%v", s.UnixSocket, err)
			}

			_ = os.MkdirAll(filepath.Dir(s.UnixSocket), 0o755)

			log.Infof("start gRPC server at unix://%s", s.UnixSocket)

			listen, err := s.buildUnixListen()
			if err != nil {
				log.Fatalf("fail to build unix listen unix://%s: %v", s.UnixSocket, err)
			}
			log.Infof("gRPC Listen at %v", listen.Addr())
			if err := s.Serve(listen); err != nil {
				log.Fatalf("failed to serve grpc server unix://%s: %v", s.UnixSocket, err)
			}
			return nil
		})
	}

	// eg的机制是只有全部的eg goroutine执行完才回去处理错误
	// 这意味着只有有一个服务正常(goroutine阻塞),这里就一直阻塞!
	// 因此不能依赖于eg的错误处理进行异常退出。
	if err := eg.Wait(); err != nil {
		log.Fatal(err.Error())
	}
}

func (s *GRPCServer) Close() {
	s.GracefulStop()
	if s.Address != "" {
		log.Infof("gRPC server on tcp://%s stopped", s.Address)
	}

	if s.UnixSocket != "" {
		_ = os.Remove(s.UnixSocket)
		log.Infof("gRPC server on unix://%s stopped", s.UnixSocket)
	}
}

// 安装通用服务的中间件和api
// 1. 这里安装的api仅会被提前安装的插件所影响
// 2. 这里安装的中间件会影响后续所有的接口。如果不希望这里有影响, 可以将中间件和通用路由特性等选项关闭。
func initGenericGRPCServer(s *GRPCServer) {
	s.InstallAPIs()
	s.InstallRuntimeDebug()
}

func (s *GRPCServer) InstallAPIs() {
	if s.Reflect {
		reflection.Register(s)
	}

	if s.Version {
		versionservice.RegisterVersionService(s.Server)
	}

	if s.Debug {
		debugservice.RegisterDebugServer(s.Server)
	}

	log.Info(
		"gRPC run with service",
		log.Bool("reflect", s.Reflect),
		log.Bool("version", s.Version),
		log.Bool("debug", s.Debug),
	)
}

func installInterceptors(interceptors []string, opt []grpc.ServerOption) []grpc.ServerOption {
	// panic recovery option
	chainUnaryInterceptor := []grpc.UnaryServerInterceptor{}
	// streamInterceptor := []grpc.StreamServerInterceptor{}

	//// 定制panic recover interceptor
	//recoveryOptions := []recovery.Option{
	//	recovery.WithRecoveryHandlerContext(recovery.CustomPanicHandler),
	//}

	// install custom interceptors
	for _, m := range interceptors {
		mw, ok := interceptor.UnaryServerInterceptorList[m]
		if !ok {
			log.Warnf("can not find  unary server interceptor: %s", m)
			continue
		}

		log.Infof("install unary server interceptors: %s", m)
		chainUnaryInterceptor = append(chainUnaryInterceptor, mw)
	}
	opt = append(opt, grpc.ChainUnaryInterceptor(chainUnaryInterceptor...))
	//opt = append(opt,grpc.ChainUnaryInterceptor(chainUnaryInterceptor...))
	//chainUnaryInterceptor = append(chainUnaryInterceptor,
	//	// otelgrpc.UnaryServerInterceptor(),
	//	requestid.UnaryServerInterceptor(),
	//	context.UnaryServerInterceptor(),
	//	logging.UnaryServerInterceptor(),
	//	recovery.UnaryServerInterceptor(recoveryOptions...),
	//)

	//streamUnaryInterceptor := []grpc.StreamServerInterceptor{}
	//streamUnaryInterceptor = append(streamUnaryInterceptor,
	//	recovery.StreamServerInterceptor(recoveryOptions...),
	//)
	return opt
}

func (s *GRPCServer) InstallRuntimeDebug() {
	if s.runtimeDebug == nil {
		return
	}

	if s.runtimeDebug.Enable {
		if s.runtimeDebug.OutputDir == "" {
			log.Warn("runtime debug output is empty")
			return
		}

		if err := os.MkdirAll(s.runtimeDebug.OutputDir, 0o755); err != nil {
			log.Warnf("mkdir runtime debug output dir `%s` err:%w", s.runtimeDebug.OutputDir, err)
			return
		}
		debug.SetupRuntimeDebugSignalHandler(s.runtimeDebug.OutputDir)
		log.Info("runtime debug start")
	}
}
