package httpconfig

import (
	"crypto/tls"

	"net/http"
	"net/url"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/tls/httptls"

	"github.com/wangweihong/gotoolbox/pkg/httpcli/httphandler"
)

const (
	DefaultTimeout = 120 * time.Second
)

type HttpConfig struct {
	// Timeout 设置整个客户端的超时, 优先级低于每个请求单独的请求
	Timeout          time.Duration
	HttpProxy        func(*http.Request) (*url.URL, error)
	HttpHandler      *httphandler.HttpHandler
	HttpTransport    *http.Transport
	TlsEnabled       bool
	SkipTlsVerified  bool
	ServerCA         string
	MutualTlsEnabled bool
	ClientKeyData    string
	ClientCertData   string
	EnableOTEL       bool
	// 记录返回的cookie,并使用
	RecordCookies bool
	// 302不自动重定向
	NoRedirect bool
}

func DefaultHttpConfig() *HttpConfig {
	return &HttpConfig{
		Timeout: DefaultTimeout,
	}
}

func (c *HttpConfig) Validate() error {
	if c.TlsEnabled {
		if !c.SkipTlsVerified {
			if c.ServerCA == "" {
				return errors.New("must set serverCA when tlsEnabled and not skipTlsVerified")
			}
		}
	}

	if c.MutualTlsEnabled {
		if c.ClientKeyData == "" || c.ClientCertData == "" {
			return errors.New("must provide clientKeyPEMData and clientCertPEMData when enable mTls")
		}

		if c.ServerCA == "" {
			return errors.New("must set serverCA when mtlsEnabled enable")
		}
	}
	return nil
}

func (c *HttpConfig) BuildCredentials() (*tls.Config, error) {
	var creds *tls.Config
	if c.TlsEnabled {
		var err error
		if c.SkipTlsVerified {
			creds = httptls.NewTlsClientSkipVerifiedCredentials()
		} else {
			if c.MutualTlsEnabled {
				// 如果开启双向认证,需要加载服务器
				creds, err = httptls.NewMutualTlsClientCredentials(
					[]byte(c.ServerCA),
					[]byte(c.ClientCertData),
					[]byte(c.ClientKeyData))
			} else {
				creds, err = httptls.NewTlsClientCredentials([]byte(c.ServerCA))
			}
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return creds, nil
}
