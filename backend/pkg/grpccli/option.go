package grpccli

import (
	"time"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"google.golang.org/grpc"
)

type Option func(*Client)

// WithTimeout 设置连接超时操作.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithReport 是否打印返参.
func WithReport() Option {
	return func(c *Client) {
		c.report = true
	}
}

// WithInsecure 是否跳过服务端证书检测.
func WithInsecure() Option {
	return func(c *Client) {
		c.tlsEnabled = true
		c.skipTlsVerified = true
	}
}

// WithServerCA 设置服务端CA证书数据.
func WithServerCA(serverCAData string) Option {
	return func(c *Client) {
		c.tlsEnabled = true
		c.serverCA = serverCAData
	}
}

// WithMTLS 是否开启双向认证.
func WithMTLS(serverCAData string, clientCertData string, clientKeyData string) Option {
	return func(c *Client) {
		c.mtlsEnabled = true
		c.tlsEnabled = true
		c.clientCertData = clientCertData
		c.clientKeyData = clientKeyData
		c.serverCA = serverCAData
	}
}

// WithCallOption 通用请求选项.
func WithCallOption(copt ...grpc.CallOption) Option {
	return func(c *Client) {
		c.callOpts = copt
	}
}

func WithIntercepts(inters ...string) Option {
	return func(c *Client) {
		c.interceptors = inters
	}
}

// WithSkippers skip interceptors.
func WithSkippers(inters ...skipper.SkipperFunc) Option {
	return func(c *Client) {
		c.interceptorSkippers = inters
	}
}

func WithDialOption(opt ...grpc.DialOption) Option {
	return func(c *Client) {
		c.dialOpts = opt
	}
}
