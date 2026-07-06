package httpcli_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/log"

	"github.com/wangweihong/gotoolbox/pkg/tracectx"

	"github.com/wangweihong/gotoolbox/pkg/maputil"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
)

var (
	inter1 = httpcli.NewInterceptor("inter1", func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client, invoker httpcli.Invoker, opts ...httpcli.CallOption) (*httpcli.HttpResponse, error) {
		req.Builder().AddQueryParam("inter1", "aaaa").Build()

		resp, err := invoker(ctx, req, arg, reply, cc, opts...)
		if err != nil {
			return resp, err
		}
		resp.Response.Header.Set("inter1", "bbbb")
		return resp, err
	})
	inter2 = httpcli.NewInterceptor("inter2", func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client, invoker httpcli.Invoker, opts ...httpcli.CallOption) (*httpcli.HttpResponse, error) {
		req.Builder().AddQueryParam("inter2", "bbbb").Build()

		ctx = context.WithValue(ctx, "inter2", "bbbb")
		resp, err := invoker(ctx, req, arg, reply, cc, opts...)
		if err != nil {
			return resp, err
		}
		resp.Response.Header.Set("inter2", "bbbb")
		return resp, err
	})
	inter3 = httpcli.NewInterceptor("errorInter", func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client, invoker httpcli.Invoker, opts ...httpcli.CallOption) (*httpcli.HttpResponse, error) {
		ctx = context.WithValue(ctx, "inter2", "bbbb")
		resp, err := invoker(ctx, req, arg, reply, cc, opts...)
		if err != nil {
			return resp, err
		}
		return resp, errors.New("intercetpor error")
	})
)

func TestClient_Interceptor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()

			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if _, err := w.Write(b); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}))
	defer server.Close()

	Convey("拦截器", t, func() {
		SkipConvey("拦截器添加查询参数，并修改返回头部", func() {

			c, err := httpcli.NewClient(nil, httpcli.WithIntercepts(inter1, inter2))
			So(err, ShouldBeNil)
			ctx := context.Background()

			os.Setenv("HTTPCLI_DEBUG", "1")
			os.Setenv("HTTPCLI_DEBUG_HUGE", "1")

			req := httpcli.NewHttpRequestBuilder().
				AddHeaderParam(tracectx.XRequestIDKey, tracectx.NewTraceID()).
				WithEndpoint(server.URL).
				WithMethod("GET").
				WithPath("/version").
				Build()
			resp, err := c.Invoke(ctx, req, nil, nil)
			So(err, ShouldBeNil)
			So(resp.Response.StatusCode, ShouldEqual, 200)
			So(resp.Response.Header.Get("inter1"), ShouldEqual, "bbbb")
			So(resp.Response.Header.Get("inter2"), ShouldEqual, "bbbb")
			So(maputil.StringAny(resp.Request.GetQueryParams()).HasKeyAndValue("inter2", "bbbb"), ShouldBeTrue)
			So(maputil.StringAny(resp.Request.GetQueryParams()).HasKeyAndValue("inter1", "aaaa"), ShouldBeTrue)
		})

		Convey("出错拦截器", func() {

			c, err := httpcli.NewClient(nil, httpcli.WithIntercepts(inter3))
			So(err, ShouldBeNil)
			ctx := context.Background()

			os.Setenv("HTTPCLI_DEBUG", "1")
			os.Setenv("HTTPCLI_DEBUG_HUGE", "1")

			req := httpcli.NewHttpRequestBuilder().
				AddHeaderParam(tracectx.XRequestIDKey, tracectx.NewTraceID()).
				WithEndpoint(server.URL).
				WithMethod("GET").
				WithPath("/version").
				Build()
			resp, err := c.Invoke(ctx, req, nil, nil)
			So(err, ShouldNotBeNil)
			So(resp, ShouldNotBeNil)
			So(resp.Response.StatusCode, ShouldEqual, 200)
			// FIXME:是否考虑移除各个中间件的log信息
			//fmt.Printf("%+v\n", err)
			//	github.com/wangweihong/gotoolbox/pkg/httpcli_test.glob..func3
			//C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/httpcli/core_test.go:53
			//	github.com/wangweihong/gotoolbox/pkg/httpcli.interceptor.Intercept
			//C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/httpcli/interceptor.go:28
			//	github.com/wangweihong/gotoolbox/pkg/httpcli.(*Client).Invoke
			//C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/httpcli/core.go:91
			//	github.com/wangweihong/gotoolbox/pkg/httpcli_test.TestClient_Interceptor.func2.2
			//C:/goprogram/src/github.com/wangweihong/gotoolbox/pkg/httpcli/core_test.go:115
		})

		SkipConvey("无拦截器", func() {

			c, err := httpcli.NewClient(nil)
			So(err, ShouldBeNil)
			ctx := context.Background()

			os.Setenv("HTTPCLI_DEBUG", "1")
			os.Setenv("HTTPCLI_DEBUG_HUGE", "1")
			ctx = context.WithValue(ctx, log.KeyRequestID, tracectx.NewTraceID())

			req := httpcli.NewHttpRequestBuilder().
				WithEndpoint(server.URL).
				WithMethod("GET").
				WithPath("/version").
				Build()
			resp, err := c.Invoke(ctx, req, nil, nil)
			So(err, ShouldBeNil)
			So(resp.Response.StatusCode, ShouldEqual, 200)
		})
	})
}
