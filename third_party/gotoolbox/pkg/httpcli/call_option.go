package httpcli

import (
	"net/http"
	"net/url"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/httpcli/decode"
)

type CallInfo struct {
	timeout            time.Duration
	httpRequestProcess func(req *http.Request) (*http.Request, error)
	httpTransport      *http.Transport
	// 拦截器列表
	chainInterceptors []Interceptor

	TlsEnabled       bool
	SkipTlsVerified  bool
	ServerCA         string
	MutualTlsEnabled bool
	ClientKeyData    string
	ClientCertData   string

	HttpProxy  func(*http.Request) (*url.URL, error)
	enableOTEL bool
	parsers    decode.ParserFactory
}

type MTLS struct {
}

type CallOption func(*CallInfo)

// TimeoutCallOption 设置某个连接超时操作.
func TimeoutCallOption(timeout time.Duration) CallOption {
	return func(c *CallInfo) {
		if timeout < 0 {
			return
		}
		c.timeout = timeout
	}
}

// CallOptionInsecure 是否跳过服务端证书检测.
func CallOptionInsecure() CallOption {
	return func(c *CallInfo) {
		c.TlsEnabled = true
		c.SkipTlsVerified = true
	}
}

// CallOptionServerCA 设置服务端CA证书数据.
func CallOptionServerCA(serverCAData string) CallOption {
	return func(c *CallInfo) {
		c.TlsEnabled = true
		c.ServerCA = serverCAData
	}
}

// CallOptionMTLS 是否开启双向认证.
func CallOptionMTLS(serverCAData string, clientCertData string, clientKeyData string) CallOption {
	return func(c *CallInfo) {
		c.MutualTlsEnabled = true
		c.TlsEnabled = true
		c.ClientCertData = clientCertData
		c.ClientKeyData = clientKeyData
		c.ServerCA = serverCAData
	}
}

type ProcessRequestFunc func(req *http.Request) (*http.Request, error)

// CallOptionHttpRequestProcess 在http请求发起调用前，对http请求进行处理. 如根据url/请求头进行加密,并写入httpReq.
func CallOptionHttpRequestProcess(fun ProcessRequestFunc) CallOption {
	return func(c *CallInfo) {
		c.httpRequestProcess = fun
	}
}

// CallOptionTransport 通用请求选项.
func CallOptionTransport(tp *http.Transport) CallOption {
	return func(c *CallInfo) {
		c.httpTransport = tp
	}
}

// CallOptionProxy 请求代理选项.
// WithProxy(http.ProxyFromEnvironment).
func CallOptionProxy(proxy func(*http.Request) (*url.URL, error)) CallOption {
	return func(c *CallInfo) {
		c.HttpProxy = proxy
	}
}

func CallOptionOTEL() CallOption {
	return func(c *CallInfo) {
		c.enableOTEL = true
	}
}

func CallOptionParserFactory(pf decode.ParserFactory) CallOption {
	return func(c *CallInfo) {
		c.parsers = pf
	}
}
