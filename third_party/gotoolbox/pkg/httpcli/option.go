package httpcli

import (
	"net/http"
	"net/url"
	"time"
)

type Option func(*Client)

// WithTimeout 设置所有连接超时操作.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.config.Timeout = timeout
	}
}

// WithInsecure 是否跳过服务端证书检测.
func WithInsecure() Option {
	return func(c *Client) {
		c.config.TlsEnabled = true
		c.config.SkipTlsVerified = true
	}
}

// WithServerCA 设置服务端CA证书数据.
func WithServerCA(serverCAData string) Option {
	return func(c *Client) {
		c.config.TlsEnabled = true
		c.config.ServerCA = serverCAData
	}
}

// WithMTLS 是否开启双向认证.
func WithMTLS(serverCAData string, clientCertData string, clientKeyData string) Option {
	return func(c *Client) {
		c.config.MutualTlsEnabled = true
		c.config.TlsEnabled = true
		c.config.ClientCertData = clientCertData
		c.config.ClientKeyData = clientKeyData
		c.config.ServerCA = serverCAData
	}
}

// WithIntercepts 插入拦截器
// 注意顺序:序号0的拦截器第一个执行调用前的处理, 最后一个执行调用后的处理.
func WithIntercepts(inters ...Interceptor) Option {
	return func(c *Client) {
		// 未避免过多混乱, 不采用追加的方式
		c.chainInterceptors = inters
	}
}

// WithTransport 通用请求选项.
func WithTransport(tp *http.Transport) Option {
	return func(c *Client) {
		c.config.HttpTransport = tp
	}
}

func WithOTEL() Option {
	return func(c *Client) {
		c.config.EnableOTEL = true
	}
}

// WithProxy 请求代理选项.
// WithProxy(http.ProxyFromEnvironment).
func WithProxy(proxy func(*http.Request) (*url.URL, error)) Option {
	return func(c *Client) {
		c.config.HttpProxy = proxy
	}
}

func WithURLProxy(proxyUrl string) Option {
	return func(c *Client) {
		proxy, err := url.Parse(proxyUrl)
		if err == nil {
			c.config.HttpProxy = http.ProxyURL(proxy)
		}
	}
}

func WithNoRedirect() Option {
	return func(c *Client) {
		c.config.NoRedirect = true
	}
}

func WithRecordCookies() Option {
	return func(c *Client) {
		c.config.RecordCookies = true
	}
}
