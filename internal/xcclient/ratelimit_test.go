package xcclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndpointRateLimiter_IsolatesEndpoints verifies that waiting on one
// endpoint does not consume tokens from another endpoint's limiter.
func TestEndpointRateLimiter_IsolatesEndpoints(t *testing.T) {
	cfg := xcclient.RateLimitConfig{
		DefaultRPS:   1,
		DefaultBurst: 1,
	}
	rl := xcclient.NewEndpointRateLimiter(cfg)

	ctx := context.Background()

	// Consume the single burst token on "origin_pools".
	require.NoError(t, rl.Wait(ctx, "origin_pools"))

	// "http_loadbalancers" has its own limiter — its burst token is still available.
	done := make(chan error, 1)
	go func() {
		done <- rl.Wait(ctx, "http_loadbalancers")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err, "http_loadbalancers should not be blocked by origin_pools")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait on http_loadbalancers timed out — endpoints are not isolated")
	}
}

// TestEndpointRateLimiter_UsesDefaults verifies that with 1 RPS / burst 1, a
// second call to the same endpoint takes roughly 1 second.
func TestEndpointRateLimiter_UsesDefaults(t *testing.T) {
	cfg := xcclient.RateLimitConfig{
		DefaultRPS:   1,
		DefaultBurst: 1,
	}
	rl := xcclient.NewEndpointRateLimiter(cfg)
	ctx := context.Background()

	// First call consumes the burst token — should be immediate.
	require.NoError(t, rl.Wait(ctx, "origin_pools"))

	start := time.Now()
	require.NoError(t, rl.Wait(ctx, "origin_pools"))
	elapsed := time.Since(start)

	// Expect roughly 1 second of delay (allow generous tolerance for CI).
	assert.GreaterOrEqual(t, elapsed, 800*time.Millisecond,
		"second call should wait ~1s for a 1 RPS limiter")
	assert.Less(t, elapsed, 3*time.Second,
		"second call should not wait longer than 3s")
}

// TestEndpointRateLimiter_OverridePerEndpoint verifies that an endpoint with
// a high-RPS override completes many calls quickly.
func TestEndpointRateLimiter_OverridePerEndpoint(t *testing.T) {
	cfg := xcclient.RateLimitConfig{
		DefaultRPS:   1,
		DefaultBurst: 1,
		Overrides: map[string]xcclient.EndpointLimit{
			"origin_pools": {RPS: 100, Burst: 10},
		},
	}
	rl := xcclient.NewEndpointRateLimiter(cfg)
	ctx := context.Background()

	start := time.Now()
	for i := 0; i < 5; i++ {
		require.NoError(t, rl.Wait(ctx, "origin_pools"))
	}
	elapsed := time.Since(start)

	// 5 calls with burst=10 should complete well under 100ms.
	assert.Less(t, elapsed, 100*time.Millisecond,
		"5 rapid calls with 100 RPS override should complete quickly")
}

// TestEndpointRateLimiter_RespectsContextCancellation verifies that a
// cancelled context causes Wait to return an error immediately.
func TestEndpointRateLimiter_RespectsContextCancellation(t *testing.T) {
	cfg := xcclient.RateLimitConfig{
		DefaultRPS:   0.1, // very slow: 1 token per 10 seconds
		DefaultBurst: 1,
	}
	rl := xcclient.NewEndpointRateLimiter(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Consume the single burst token.
	require.NoError(t, rl.Wait(ctx, "slow_endpoint"))

	// Cancel the context before the next token would be available.
	cancel()

	err := rl.Wait(ctx, "slow_endpoint")
	require.Error(t, err, "Wait should return error when context is cancelled")
	assert.ErrorIs(t, err, context.Canceled)
}
