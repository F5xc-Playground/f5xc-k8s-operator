package xcclient

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// EndpointRateLimiter manages per-endpoint token-bucket rate limiters.
// Each unique endpoint string gets its own rate.Limiter so that one slow
// endpoint cannot starve requests to another.
type EndpointRateLimiter struct {
	config   RateLimitConfig
	limiters sync.Map // map[string]*rate.Limiter
}

// NewEndpointRateLimiter returns an EndpointRateLimiter configured with cfg.
func NewEndpointRateLimiter(cfg RateLimitConfig) *EndpointRateLimiter {
	return &EndpointRateLimiter{config: cfg}
}

// Wait blocks until a token is available for endpoint, or until ctx is done.
// It returns ctx.Err() if the context is cancelled before a token is granted.
func (e *EndpointRateLimiter) Wait(ctx context.Context, endpoint string) error {
	return e.getLimiter(endpoint).Wait(ctx)
}

// getLimiter returns the rate.Limiter for endpoint, creating one on first use.
// Per-endpoint overrides in config.Overrides take precedence over the defaults.
func (e *EndpointRateLimiter) getLimiter(endpoint string) *rate.Limiter {
	if v, ok := e.limiters.Load(endpoint); ok {
		return v.(*rate.Limiter)
	}

	rps := e.config.DefaultRPS
	burst := e.config.DefaultBurst
	if ov, found := e.config.Overrides[endpoint]; found {
		rps = ov.RPS
		burst = ov.Burst
	}

	newL := rate.NewLimiter(rate.Limit(rps), burst)
	actual, _ := e.limiters.LoadOrStore(endpoint, newL)
	return actual.(*rate.Limiter)
}
