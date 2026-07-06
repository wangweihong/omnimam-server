package grpcoptions

import (
	"fmt"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/debug"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	"github.com/spf13/pflag"

	"github.com/wangweihong/gotoolbox/pkg/sets"

	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr/interceptor"

	"github.com/wangweihong/omnimam/backend/pkg/grpcsvr"
)

// ServerRunOptions contains the options while running a generic gRPC server.
type ServerRunOptions struct {
	MaxMsgSize         int      `json:"max-msg-size"        mapstructure:"max-msg-size"`
	Version            bool     `json:"version"             mapstructure:"version"`             // 开启版本服务
	Reflect            bool     `json:"reflect"             mapstructure:"reflect"`             // 是否开启gRPC反射服务。开启反射服务后, grpcurl工具才能获取gRPC服务接口
	Debug              bool     `json:"debug"               mapstructure:"debug"`               // 是否开启调试服务
	UnaryInterceptors  []string `json:"unary-interceptors"  mapstructure:"unary-interceptors"`  // 启动拦截器列表
	StreamInterceptors []string `json:"stream-interceptors" mapstructure:"stream-interceptors"` // 启动拦截器列表

	RuntimeDebug    bool   `json:"runtime-debug"     mapstructure:"runtime-debug"`     // 开启运行时调试
	RuntimeDebugDir string `json:"runtime-debug-dir" mapstructure:"runtime-debug-dir"` // 调试输出目录
}

// NewServerRunOptions creates a new ServerRunOptions object with default parameters.
func NewServerRunOptions() *ServerRunOptions {
	defaults := grpcsvr.NewConfig()

	return &ServerRunOptions{
		MaxMsgSize:         4 * 1024 * 1024,
		Version:            defaults.Version,
		Reflect:            defaults.Reflect,
		Debug:              defaults.Debug,
		UnaryInterceptors:  defaults.UnaryInterceptors,
		StreamInterceptors: defaults.StreamInterceptors,
		RuntimeDebug:       defaults.RuntimeDebug.Enable,
		RuntimeDebugDir:    defaults.RuntimeDebug.OutputDir,
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
func (s *ServerRunOptions) ApplyTo(c *grpcsvr.GRPCConfig) error {
	c.MaxMsgSize = s.MaxMsgSize
	c.Version = s.Version
	c.Reflect = s.Reflect
	c.Debug = s.Debug
	c.UnaryInterceptors = s.UnaryInterceptors
	c.StreamInterceptors = s.StreamInterceptors
	c.RuntimeDebug = &debug.RuntimeDebugInfo{
		Enable:    s.RuntimeDebug,
		OutputDir: s.RuntimeDebugDir,
	}
	return nil
}

// Validate checks validation of ServerRunOptions.
func (s *ServerRunOptions) Validate() []error {
	errors := []error{}

	rm, repeated := sliceutil.StringSlice(s.UnaryInterceptors).GetRepeat()
	if repeated {
		errors = append(errors, fmt.Errorf("unary interceptors `%v` is repeated", maputil.StringInt(rm).Keys()))
	}

	supportedUnaryInterceptor := sets.NewString(interceptor.UnaryServerInterceptorNames...)
	if !supportedUnaryInterceptor.HasAll(s.UnaryInterceptors...) {
		invalidInterceptors := sets.NewString(s.UnaryInterceptors...).Difference(supportedUnaryInterceptor)
		errors = append(errors, fmt.Errorf("unary intercerptor `%v` is not supported", invalidInterceptors.List()))
	}

	if s.RuntimeDebug {
		if s.RuntimeDebugDir == "" {
			errors = append(errors, fmt.Errorf("set `RuntimeDebugDir` when enable runtime debug"))
		}
	}
	return errors
}

// AddFlags adds flags for a specific APIServer to the specified FlagSet.
func (s *ServerRunOptions) AddFlags(fs *pflag.FlagSet) {
	// Note: the weird ""+ in below lines seems to be the only way to get gofmt to
	// arrange these text blocks sensibly. Grrr.
	fs.BoolVar(&s.Version, "server.version", s.Version, ""+
		"Install version service.")

	fs.BoolVar(&s.Reflect, "server.reflect", s.Reflect, ""+
		"Whether enable gRPC server register reflect service. If registered, grpc client can "+
		"get gRPC service list directly. ")

	fs.BoolVar(&s.Debug, "server.debug", s.Debug, ""+
		"Install debug service.")

	fs.StringSliceVar(
		&s.UnaryInterceptors,
		"server.unary-interceptors",
		s.UnaryInterceptors,
		"List of allowed unary interceptors for server, comma separated. If this list is empty,no unary interceptors will be used."+
			"Support unary interceptors: "+strings.Join(
			interceptor.UnaryServerInterceptorNames,
			",",
		),
	)
	fs.IntVar(&s.MaxMsgSize, "server.max-msg-size", s.MaxMsgSize, "gRPC max message size.")

	fs.BoolVar(&s.RuntimeDebug, "server.runtime-debug", s.RuntimeDebug, ""+
		"Enable debugging during runtime.")

	fs.StringVar(&s.RuntimeDebugDir, "server.runtime-debug-dir", s.RuntimeDebugDir, ""+
		"Directory runtime debug data saved")
}
