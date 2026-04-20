package xcclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleRateLimiterCreate returns a minimal XCRateLimiterCreate for use in tests.
func sampleRateLimiterCreate(name string) xcclient.XCRateLimiterCreate {
	return xcclient.XCRateLimiterCreate{
		Metadata: xcclient.ObjectMeta{Name: name},
		Spec: xcclient.XCRateLimiterSpec{
			Threshold:       100,
			Unit:            "MINUTE",
			BurstMultiplier: 2,
		},
	}
}

// TestRateLimiter_CreateAndGet verifies that a created rate limiter can be
// retrieved and that the returned metadata and RawSpec match expectations.
func TestRateLimiter_CreateAndGet(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	created, err := c.CreateRateLimiter(ctx, ns, sampleRateLimiterCreate("my-rl"))
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "my-rl", created.Metadata.Name)
	assert.Equal(t, ns, created.Metadata.Namespace)

	got, err := c.GetRateLimiter(ctx, ns, "my-rl")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "my-rl", got.Metadata.Name)
	assert.Equal(t, ns, got.Metadata.Namespace)
	// Verify RawSpec is populated.
	assert.NotEmpty(t, got.RawSpec)
}

// TestRateLimiter_DeleteAndVerifyGone verifies that after deleting a rate
// limiter a subsequent Get returns ErrNotFound.
func TestRateLimiter_DeleteAndVerifyGone(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	_, err := c.CreateRateLimiter(ctx, ns, sampleRateLimiterCreate("rl-to-delete"))
	require.NoError(t, err)

	err = c.DeleteRateLimiter(ctx, ns, "rl-to-delete")
	require.NoError(t, err)

	_, err = c.GetRateLimiter(ctx, ns, "rl-to-delete")
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"expected ErrNotFound after delete, got %v", err)
}

// TestRateLimiter_ListCount verifies that listing returns the correct number
// of rate limiters in a namespace.
func TestRateLimiter_ListCount(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	for _, name := range []string{"rl-a", "rl-b", "rl-c"} {
		_, err := c.CreateRateLimiter(ctx, ns, sampleRateLimiterCreate(name))
		require.NoError(t, err)
	}

	rls, err := c.ListRateLimiters(ctx, ns)
	require.NoError(t, err)
	assert.Len(t, rls, 3)
}
