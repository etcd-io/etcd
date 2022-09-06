package etcdmain

import "golang.org/x/time/rate"

type RateLimiter struct {
	limiter rate.Limiter
}

func NewRateLimiter(limit, burst int) *RateLimiter {
	return &RateLimiter{
		limiter: *rate.NewLimiter(rate.Limit(limit), burst),
	}
}

func (r *RateLimiter) Limit() bool {
	return r.limiter.Allow()
}
