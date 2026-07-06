package httpsvr

import (
	"net"
	"strconv"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/debug"

	"github.com/wangweihong/gotoolbox/pkg/tls"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"

	"github.com/gin-gonic/gin"
)

// Config is a structure used to configure a GenericServer.
// Its members are sorted roughly in order of importance for composers.
type Config struct {
	SecureServing   *SecureServingInfo
	InsecureServing *InsecureServingInfo
	Jwt             *JwtInfo
	Mode            string
	Middlewares     []string
	Healthz         bool
	Version         bool

	EnableMetrics bool
	Profiling     *FeatureProfilingInfo
	RuntimeDebug  *debug.RuntimeDebugInfo
}

// SecureServingInfo holds configuration of the TLS server.
type SecureServingInfo struct {
	BindAddress string
	BindPort    int
	CertKey     tls.CertData
	Required    bool
}

// Address join host IP address and host port number into a address string, like: 0.0.0.0:8443.
func (s *SecureServingInfo) Address() string {
	return net.JoinHostPort(s.BindAddress, strconv.Itoa(s.BindPort))
}

// InsecureServingInfo holds configuration of the insecure http server.
type InsecureServingInfo struct {
	Address  string
	Required bool
}

// JwtInfo defines jwt fields used to create jwt authentication middleware.
type JwtInfo struct {
	Realm string
	// defaults to empty
	Key string
	// defaults to one hour
	Timeout time.Duration
	// defaults to zero
	MaxRefresh time.Duration
}

type FeatureProfilingInfo struct {
	// enable profiling
	EnableProfiling bool
	// standalone profiling apis
	StandAloneProfiling bool
	// standalone profiling address
	ProfileAddress string
}

// NewConfig returns a Config struct with the default values.
func NewConfig() *Config {
	return &Config{
		Version: true,
		Healthz: true,
		Mode:    gin.ReleaseMode,
		Middlewares: []string{
			genericmiddleware.MWNameRequestID,
			genericmiddleware.MWNameContext,
		},
		EnableMetrics: true,
		Jwt: &JwtInfo{
			Realm:      "jwt",
			Timeout:    1 * time.Hour,
			MaxRefresh: 1 * time.Hour,
		},
		Profiling: &FeatureProfilingInfo{
			EnableProfiling:     true,
			StandAloneProfiling: false,
			ProfileAddress:      "127.0.0.1:6060",
		},
		RuntimeDebug: &debug.RuntimeDebugInfo{
			Enable:    false,
			OutputDir: "",
		},
		InsecureServing: &InsecureServingInfo{},
		SecureServing:   &SecureServingInfo{},
	}
}

// CompletedConfig is the completed configuration for GenericAPIServer.
type CompletedConfig struct {
	*Config
}

// Complete fills in any fields not set that are required to have valid data and can be derived
// from other fields. If you're going to `ApplyOptions`, do that first. It's mutating the receiver.
func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{c}
}

// New returns a new instance of GenericAPIServer from the given config.
func (c CompletedConfig) New() (*GenericHTTPServer, error) {
	gin.SetMode(c.Mode)

	s := &GenericHTTPServer{
		SecureServingInfo:   c.SecureServing,
		InsecureServingInfo: c.InsecureServing,
		healthz:             c.Healthz,
		version:             c.Version,
		enableMetrics:       c.EnableMetrics,
		profiling:           c.Profiling,
		middlewares:         c.Middlewares,
		Engine:              gin.New(),
		runtimeDebug:        c.RuntimeDebug,
	}

	// 初始化http server配置
	// 1. 安装通用的中间件
	// 2. 安装通用的路由, 如版本,健康,pprof等
	initGenericHTTPServer(s)

	return s, nil
}
