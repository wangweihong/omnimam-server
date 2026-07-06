package grpccli

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/skipper"

	"github.com/wangweihong/omnimam/backend/pkg/grpccli/interceptorcli"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/tls/grpctls"
)

// type CallerHandler func(ctx context.Context, conn *grpc.ClientConn) (any, error).
type CallerHandler func(ctx context.Context, conn *grpc.ClientConn) error

type Client struct {
	conn            *grpc.ClientConn
	timeout         time.Duration
	addr            string
	report          bool
	tlsEnabled      bool
	skipTlsVerified bool
	// gRPC服务证书
	serverCA       string
	mtlsEnabled    bool
	clientKeyData  string
	clientCertData string
	// 通用调用选项。如请求传递追踪ID等
	callOpts []grpc.CallOption
	// 通用连接选项。如设置拦截器等
	dialOpts []grpc.DialOption
	// 拦截器列表
	interceptors        []string
	interceptorSkippers []skipper.SkipperFunc
}

func NewClient(addr string, options ...Option) (*Client, error) {
	c := &Client{
		addr:     addr,
		callOpts: make([]grpc.CallOption, 0),
	}
	for _, o := range options {
		o(c)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Errorf("close connection of addr %v err:%v", c.addr, err)
		}
		c.conn = nil
	}
}

func (c *Client) validate() error {
	if c.addr == "" {
		return fmt.Errorf("client addr is empty")
	}

	if c.tlsEnabled {
		if !c.skipTlsVerified {
			if c.serverCA == "" {
				return fmt.Errorf("must set serverCA when tlsEnabled and not skipTlsVerified")
			}
		}
	}

	if c.mtlsEnabled {
		if c.clientKeyData == "" || c.clientCertData == "" {
			return fmt.Errorf("must provide clientKeyPEMData and clientCertPEMData when enable mTls")
		}

		if c.serverCA == "" {
			return fmt.Errorf("must set serverCA when mtlsEnabled enable")
		}
	}
	return nil
}

func (c *Client) Call(ctx context.Context, call CallerHandler) error {
	if c.timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	conn, err := c.getClientConn(ctx, c.addr, c.callOpts...)
	if err != nil {
		return errors.WithStack(err)
	}
	// defer conn.Close()
	if err := call(ctx, conn); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (c *Client) GetConn(ctx context.Context) (*grpc.ClientConn, error) {
	return c.getClientConn(ctx, c.addr, c.callOpts...)
}

func (c *Client) getClientConn(ctx context.Context, addr string, copt ...grpc.CallOption) (*grpc.ClientConn, error) {
	// reuse conn
	// lock?
	if c.conn != nil {
		log.F(ctx).Debug("connection has exist, reuse it.")
		return c.conn, nil
	}

	var creds credentials.TransportCredentials
	var err error

	if c.tlsEnabled {
		if c.skipTlsVerified {
			creds, err = grpctls.NewTlsClientSkipVerifiedCredentials()
		} else {
			if c.mtlsEnabled {
				// 如果开启双向认证,需要加载服务器
				creds, err = grpctls.NewMutualTlsClientCredentials([]byte(c.serverCA), []byte(c.clientCertData), []byte(c.clientKeyData))
			} else {
				creds, err = grpctls.NewTlsClientCredentials([]byte(c.serverCA))
			}
		}
		if err != nil {
			return nil, errors.Wrap(err, "generate tls credential fail")
		}
	} else {
		creds = insecure.NewCredentials()
	}

	chainInterceptors := make([]grpc.UnaryClientInterceptor, 0, len(c.interceptors))
	for _, m := range c.interceptors {
		mw, ok := interceptorcli.GetUnaryClientInterceptorWithSkippers(c.interceptorSkippers...)[m]
		if !ok {
			log.F(ctx).Warnf("can not find  unary client interceptor: %s", m)
			continue
		}

		log.F(ctx).Debugf("install unary client interceptors: %s", m)
		chainInterceptors = append(chainInterceptors, mw)
	}

	var opt []grpc.DialOption
	opt = append(opt,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(copt...),
		grpc.WithChainUnaryInterceptor(chainInterceptors...),
	)
	// custom dial options
	opt = append(opt, c.dialOpts...)

	conn, err := grpc.NewClient(addr, opt...)
	if err != nil {
		return nil, errors.Wrapf(err, "dial to addr %s error", addr)
	}

	c.conn = conn
	return conn, nil
}
