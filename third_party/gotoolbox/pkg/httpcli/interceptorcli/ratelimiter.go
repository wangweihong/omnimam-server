package interceptorcli

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/rate"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/skipper"
)

// rps: 每秒请求数限制
// burst: 突发请求容量
func NewPathBasedRateLimiter(rps, burst int) *PathBasedRateLimiter {
	return &PathBasedRateLimiter{
		defaultLimiter: rate.NewLimiter(rate.Limit(rps), burst),
		pathLimiters:   make(map[string]*rate.Limiter),
	}
}

type PathBasedRateLimiter struct {
	defaultLimiter *rate.Limiter
	pathLimiters   map[string]*rate.Limiter
}

func (r *PathBasedRateLimiter) UpdatePath(path string, rps, burst int) {
	r.pathLimiters[path] = rate.NewLimiter(rate.Limit(rps), burst)
}

func RateLimitInterceptor(name string, p *PathBasedRateLimiter, skipperFunc ...skipper.SkipperFunc) httpcli.Interceptor {
	return httpcli.NewInterceptor(name, func(ctx context.Context, req *httpcli.HttpRequest, arg, reply any, cc *httpcli.Client,
		invoker httpcli.Invoker, opts ...httpcli.CallOption) (*httpcli.HttpResponse, error) {
		if skipper.Skip(req.GetPath(), skipperFunc...) {
			log.F(ctx).Debugf("skip interceptor %s for rawrurl %s", name, req.GetPath())

			return invoker(ctx, req, arg, reply, cc, opts...)
		}

		if p != nil {
			path := req.GetPath()

			limiter, exists := p.pathLimiters[path]
			if !exists {
				limiter = p.defaultLimiter
			}

			err := limiter.Wait(ctx)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		rawResp, err := invoker(ctx, req, arg, reply, cc, opts...)
		return rawResp, errors.WithStack(err)

	})
}
