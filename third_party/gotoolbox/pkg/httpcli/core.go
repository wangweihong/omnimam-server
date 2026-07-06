package httpcli

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/callerutil"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/httpcli/httpconfig"
)

type Client struct {
	conn              *http.Client
	config            httpconfig.HttpConfig
	chainInterceptors []Interceptor
}

func NewClient(cfg *httpconfig.HttpConfig, options ...Option) (*Client, error) {
	config := httpconfig.DefaultHttpConfig()
	if cfg != nil {
		config = cfg
	}

	c := &Client{
		config: *config,
	}
	for _, o := range options {
		o(c)
	}

	tr := &http.Transport{}
	if c.config.HttpTransport != nil {
		tr = c.config.HttpTransport
	} else {
		creds, err := c.config.BuildCredentials()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		tr.Proxy = c.config.HttpProxy
		tr.TLSClientConfig = creds
	}

	c.conn = &http.Client{
		Transport: tr,
		Timeout:   c.config.Timeout,
	}

	if c.config.RecordCookies {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		c.conn.Jar = jar
	}

	if c.config.NoRedirect {
		c.conn.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			// 返回错误以阻止重定向
			return http.ErrUseLastResponse
		}
	}

	return c, nil
}

func (c *Client) Invoke(
	/*
			注意1. ctx的生命周期,如果Client在一个服务请求的异步动作,不能直接使用服务请求的ctx。否则当服务器结束后,context撤销
		会导致所有Client的请求都会立即`context cancel`失败返回
			注意2. log fieldCtx的信息覆盖问题. 如果ctx来自一个服务请求,服务的中间件可能在该请求中设置了请求相关信息。如果client
		也使用这个ctx, 在记录信息，注意字段的覆盖和冗余问题.
	*/
	ctx context.Context,
	req *HttpRequest,
	arg, reply any,
	opts ...CallOption,
) (*HttpResponse, error) {

	var err error
	var rawResp *HttpResponse

	//FIXME: 是否采用拦截器来处理?
	if logEnabled() {
		caller := map[string]any{"caller": callerutil.CallersDepth(20, 3).List()}
		//debugLog(ctx, caller, "Client Invoke Called")
		defer func() {
			caller["err"] = errors.Message(err)
			//	debugLog(ctx, caller, "Client Invoke Ended")
		}()
	}

	ci := &CallInfo{}
	for _, o := range opts {
		o(ci)
	}

	//  允许特定请求单独设置拦截器来覆盖客户端的拦截器
	chainInterceptors := c.chainInterceptors
	if ci.chainInterceptors != nil {
		chainInterceptors = ci.chainInterceptors
	}

	if chainInterceptors != nil {
		rawResp, err = chainInterceptors[0].Intercept(
			ctx,
			req,
			arg,
			reply,
			c,
			getChainUnaryInvoker(chainInterceptors, 0, invokeLogWrapper),
			opts...)

		return rawResp, errors.WithStack(err)
	}

	rawResp, err = invokeLogWrapper(ctx, req, arg, reply, c, opts...)
	return rawResp, errors.WithStack(err)
}

func getChainUnaryInvoker(interceptors []Interceptor, curr int, finalInvoker Invoker) Invoker {
	if curr == len(interceptors)-1 {
		return finalInvoker
	}
	return func(ctx context.Context, req *HttpRequest, arg, reply any, cc *Client, opts ...CallOption) (*HttpResponse, error) {
		rawResp, err := interceptors[curr+1].Intercept(
			ctx,
			req,
			arg,
			reply,
			cc,
			getChainUnaryInvoker(interceptors, curr+1, finalInvoker),
			opts...)
		return rawResp, err
	}
}

type Invoker func(ctx context.Context, req *HttpRequest, arg, reply any, cc *Client, opt ...CallOption) (*HttpResponse, error)

// nolint: funlen,gocognit
func invokeLogWrapper(
	ctx context.Context,
	req *HttpRequest,
	arg any,
	reply any,
	c *Client,
	opt ...CallOption,
) (*HttpResponse, error) {
	startTime := time.Now()
	logInfoIf(ctx, "Core called.")
	httpResp, err := invoke(ctx, req, arg, reply, c, opt...)
	debugCore(ctx, startTime, req, httpResp, arg, reply, err)
	logInfoIf(ctx, "Core end")

	return httpResp, errors.WithStack(err)
}

// nolint: funlen,gocognit
func invoke(
	ctx context.Context,
	req *HttpRequest,
	arg any,
	reply any,
	c *Client,
	opt ...CallOption,
) (*HttpResponse, error) {
	ci := &CallInfo{}
	for _, o := range opt {
		o(ci)
	}

	// refer to https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	var cancel context.CancelFunc
	// 如果某个请求指定的超时时间, 则采用该超时时间
	if ci.timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, ci.timeout)
		defer cancel()
	}

	// 转换成原生http请求
	httpReq, err := req.ConvertRequestWithContext(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// 请求前处理. 如一些场景需要对请求参数进行签名
	if err := c.preRequestProcess(httpReq, ci); err != nil {
		return nil, errors.WithStack(err)
	}

	resp, err := c.conn.Do(httpReq)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// 请求后处理
	if err := c.postRequestProcess(resp, ci); err != nil {
		return nil, errors.WithStack(err)
	}

	return NewHttpResponse(req, resp), nil
}

func (c *Client) preRequestProcess(req *http.Request, info *CallInfo) error {
	if info != nil && info.httpRequestProcess != nil {
		if _, err := info.httpRequestProcess(req); err != nil {
			return errors.Wrap(err, "process request before invoke fail")
		}
	}

	if c.config.HttpHandler == nil || c.config.HttpHandler.RequestHandlers == nil || req == nil {
		return nil
	}
	return c.config.HttpHandler.RequestHandlers(req)
}

func (c *Client) postRequestProcess(resp *http.Response, info *CallInfo) error {
	if c.config.HttpHandler == nil || c.config.HttpHandler.ResponseHandlers == nil || resp == nil {
		return nil
	}

	return c.config.HttpHandler.ResponseHandlers(resp)
}
