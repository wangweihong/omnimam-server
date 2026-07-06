package httpcli

import (
	"context"
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

type InterceptFunc func(ctx context.Context, req *HttpRequest, arg, reply any, cc *Client, invoker Invoker, opts ...CallOption) (*HttpResponse, error)

type Interceptor interface {
	Intercept(ctx context.Context, req *HttpRequest, arg, reply any, cc *Client, invoker Invoker, opts ...CallOption) (*HttpResponse, error)
	Name() string
}

type interceptor struct {
	name      string
	interFunc InterceptFunc
}

func (i interceptor) Intercept(ctx context.Context, req *HttpRequest, arg,
	reply any, cc *Client, invoker Invoker, opts ...CallOption) (*HttpResponse, error) {
	startTime := time.Now()
	logInfoIf(ctx, fmt.Sprintf("Interceptor '%v' Invoked called.", i.Name()))

	rawResp, err := i.interFunc(ctx, req, arg, reply, cc, invoker, opts...)

	debugCore(ctx, startTime, req, rawResp, arg, reply, err)
	logInfoIf(ctx, fmt.Sprintf("Interceptor '%v' Invoked end.", i.Name()))

	return rawResp, errors.WithStack(err)

}

func (i interceptor) Name() string {
	return i.name
}

func NewInterceptor(name string, interFunc InterceptFunc) Interceptor {
	return interceptor{
		name:      name,
		interFunc: interFunc,
	}
}
