package grpcsvr_test

import (
	"context"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/wangweihong/omnimam/backend/pkg/grpcproto/apis/debug"
	"github.com/wangweihong/omnimam/backend/pkg/grpcproto/apis/version"
	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr"
)

func testUnixSocket(conf *grpcsvr.GRPCConfig, addr string) {
	unixSocketInstall(conf, "unix://"+conf.UnixSocket)
}

func unixSocketInstall(conf *grpcsvr.GRPCConfig, addr string) {
	s := installServer(conf)
	defer s.Stop()

	// Set up a gRPC connection to the server
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	So(err, ShouldBeNil)

	defer conn.Close()

	_, err = debug.NewDebugServiceClient(conn).
		Sleep(context.Background(), &debug.SleepRequest{Duration: durationpb.New(50 * time.Millisecond)})

	if conf.Debug {
		So(err, ShouldBeNil)
	} else {
		So(err, ShouldNotBeNil)
	}

	versionResp, err := version.NewVersionServiceClient(conn).Version(context.Background(), &version.VersionRequest{})
	if conf.Version {
		So(err, ShouldBeNil)
		So(versionResp, ShouldNotBeNil)
	} else {
		So(err, ShouldNotBeNil)
	}
}
