package xcclient

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// EndpointRateLimiter manages per-endpoint token-bucket rate limiters.
type EndpointRateLimiter struct {
	cfg      RateLimitConfig
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

// NewEndpointRateLimiter creates an EndpointRateLimiter backed by cfg.
func NewEndpointRateLimiter(cfg RateLimitConfig) *EndpointRateLimiter {
	return &EndpointRateLimiter{
		cfg:      cfg,
		limiters: make(map[string]*rate.Limiter),
	}
}

// Wait blocks until a token is available for the given endpoint, or ctx is done.
func (e *EndpointRateLimiter) Wait(ctx context.Context, endpoint string) error {
	e.mu.Lock()
	l, ok := e.limiters[endpoint]
	if !ok {
		rps := e.cfg.DefaultRPS
		burst := e.cfg.DefaultBurst
		if ov, found := e.cfg.Overrides[endpoint]; found {
			rps = ov.RPS
			burst = ov.Burst
		}
		l = rate.NewLimiter(rate.Limit(rps), burst)
		e.limiters[endpoint] = l
	}
	e.mu.Unlock()
	return l.Wait(ctx)
}
