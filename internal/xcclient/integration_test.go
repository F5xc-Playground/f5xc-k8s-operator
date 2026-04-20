package xcclient_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRetryClient builds a *Client with the given maxRetries and a small base
// delay so that retry loops complete quickly in tests.
func newRetryClient(t *testing.T, url string, maxRetries int) *xcclient.Client {
	t.Helper()
	cfg := xcclient.Config{
		TenantURL:  url,
		APIToken:   "test-token",
		MaxRetries: maxRetries,
	}
	c, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	c.SetBaseDelay(10 * time.Millisecond)
	return c
}

// integrationPool returns a minimal OriginPoolCreate for integration tests.
func integrationPool(name string) *xcclient.OriginPoolCreate {
	return &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{Name: name},
		Spec: xcclient.OriginPoolSpec{
			OriginServers: []xcclient.OriginServer{
				{PublicIP: &xcclient.PublicIP{IP: "10.0.0.1"}},
			},
			Port: 80,
		},
	}
}

// TestIntegration_429RetryThenSuccess verifies that the client retries on 429
// responses and ultimately succeeds when the server stops rate-limiting.
// The fake server injects 2 consecutive 429 errors; the 3rd attempt succeeds.
// We expect exactly 3 POST requests to be recorded.
func TestIntegration_429RetryThenSuccess(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	// Inject 2 x 429 on POST origin_pools in namespace "default" (no name on
	// create — the endpoint key uses empty string for name).
	srv.InjectError(http.MethodPost, xcclient.ResourceOriginPool, "default", "", testutil.ErrorSpec{
		StatusCode: http.StatusTooManyRequests,
		Body:       `{"message":"rate limited"}`,
		Times:      2,
	})

	// Use MaxRetries=3 so the client can retry past the two 429s.
	c := newRetryClient(t, srv.URL(), 3)
	ctx := context.Background()

	pool, err := c.CreateOriginPool(ctx, "default", integrationPool("retry-pool"))
	require.NoError(t, err, "create should succeed on the 3rd attempt")
	assert.Equal(t, "retry-pool", pool.Metadata.Name)

	reqs := srv.Requests()
	// Filter to only POST requests so we are not sensitive to unrelated traffic.
	var posts int
	for _, r := range reqs {
		if r.Method == http.MethodPost {
			posts++
		}
	}
	assert.Equal(t, 3, posts, "expected 3 POST attempts (2 x 429 + 1 success)")
}

// TestIntegration_409ConflictOnDuplicateCreate verifies that creating an origin
// pool with a name that already exists returns ErrConflict.
func TestIntegration_409ConflictOnDuplicateCreate(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	// First create should succeed.
	_, err := c.CreateOriginPool(ctx, ns, integrationPool("dup-pool"))
	require.NoError(t, err, "first create should succeed")

	// Second create of the same name should return ErrConflict.
	_, err = c.CreateOriginPool(ctx, ns, integrationPool("dup-pool"))
	require.Error(t, err, "second create should fail with conflict")
	assert.True(t, errors.Is(err, xcclient.ErrConflict),
		"expected ErrConflict on duplicate create, got %v", err)
}

// TestIntegration_DeleteIdempotent verifies that deleting a nonexistent pool
// returns ErrNotFound, which callers should treat as a success (idempotent).
func TestIntegration_DeleteIdempotent(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()

	err := c.DeleteOriginPool(ctx, "default", "ghost-pool")
	require.Error(t, err, "deleting nonexistent pool should return an error")
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"deleting nonexistent pool should return ErrNotFound, got %v", err)
}

// TestIntegration_FullCRUDLifecycle exercises the complete Create → Get →
// Replace → List → Delete → Get sequence and verifies the state at each step.
func TestIntegration_FullCRUDLifecycle(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"
	name := "lifecycle-pool"

	// Create
	created, err := c.CreateOriginPool(ctx, ns, integrationPool(name))
	require.NoError(t, err, "create should succeed")
	require.NotNil(t, created)
	assert.Equal(t, name, created.Metadata.Name)
	assert.Equal(t, ns, created.Metadata.Namespace)

	// Get — pool must be retrievable after create.
	got, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err, "get after create should succeed")
	require.NotNil(t, got)
	assert.Equal(t, name, got.Metadata.Name)
	assert.NotEmpty(t, got.RawSpec, "RawSpec must be populated")

	// Replace — update the spec.
	replacement := &xcclient.OriginPoolReplace{
		Metadata: xcclient.ObjectMeta{Name: name},
		Spec: xcclient.OriginPoolSpec{
			OriginServers: []xcclient.OriginServer{
				{PublicIP: &xcclient.PublicIP{IP: "192.168.1.1"}},
			},
			Port: 8443,
		},
	}
	replaced, err := c.ReplaceOriginPool(ctx, ns, name, replacement)
	require.NoError(t, err, "replace should succeed")
	require.NotNil(t, replaced)
	assert.Equal(t, name, replaced.Metadata.Name)

	// List — expect exactly 1 pool in the namespace.
	pools, err := c.ListOriginPools(ctx, ns)
	require.NoError(t, err, "list should succeed")
	assert.Len(t, pools, 1, "expected exactly 1 pool after create+replace")

	// Delete
	err = c.DeleteOriginPool(ctx, ns, name)
	require.NoError(t, err, "delete should succeed")

	// Get after delete must return ErrNotFound.
	_, err = c.GetOriginPool(ctx, ns, name)
	require.Error(t, err, "get after delete should return an error")
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"get after delete should return ErrNotFound, got %v", err)
}

// TestIntegration_5xxNotRetried verifies that 5xx errors are not retried — the
// client should record exactly 1 request regardless of MaxRetries.
func TestIntegration_5xxNotRetried(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	// Inject a permanent 500 on GET for a specific pool name.
	srv.InjectError(http.MethodGet, xcclient.ResourceOriginPool, "default", "bad-pool", testutil.ErrorSpec{
		StatusCode: http.StatusInternalServerError,
		Body:       `{"message":"internal server error"}`,
		Times:      0, // fire forever
	})

	// Use MaxRetries=3 — the client would retry 429s but should NOT retry 500s.
	c := newRetryClient(t, srv.URL(), 3)
	ctx := context.Background()

	_, err := c.GetOriginPool(ctx, "default", "bad-pool")
	require.Error(t, err, "GET with injected 500 should fail")
	assert.True(t, errors.Is(err, xcclient.ErrServerError),
		"expected ErrServerError, got %v", err)

	// Count only GET requests to the target pool.
	var gets int
	for _, r := range srv.Requests() {
		if r.Method == http.MethodGet {
			gets++
		}
	}
	assert.Equal(t, 1, gets, "5xx must not be retried — expected exactly 1 GET request")
}
