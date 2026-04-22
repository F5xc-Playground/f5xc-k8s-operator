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

// sampleOriginPoolCreate returns a minimal OriginPoolCreate for use in tests.
func sampleOriginPoolCreate(name string) *xcclient.OriginPoolCreate {
	return &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{Name: name},
		Spec: xcclient.OriginPoolSpec{
			OriginServers: []xcclient.OriginServer{
				{PublicIP: &xcclient.PublicIP{IP: "1.2.3.4"}},
			},
			Port:                  443,
			LoadBalancerAlgorithm: "ROUND_ROBIN",
		},
	}
}

// TestOriginPool_CreateAndGet verifies that a created pool can be retrieved
// and that the returned metadata matches what was sent.
func TestOriginPool_CreateAndGet(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	created, err := c.CreateOriginPool(ctx, ns, sampleOriginPoolCreate("my-pool"))
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "my-pool", created.Metadata.Name)
	assert.Equal(t, ns, created.Metadata.Namespace)

	got, err := c.GetOriginPool(ctx, ns, "my-pool")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "my-pool", got.Metadata.Name)
	assert.Equal(t, ns, got.Metadata.Namespace)
	// Verify RawSpec is populated.
	assert.NotEmpty(t, got.RawSpec)
}

// TestOriginPool_Replace verifies that Replace updates an existing pool.
func TestOriginPool_Replace(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	_, err := c.CreateOriginPool(ctx, ns, sampleOriginPoolCreate("replace-me"))
	require.NoError(t, err)

	replacement := &xcclient.OriginPoolReplace{
		Metadata: xcclient.ObjectMeta{Name: "replace-me"},
		Spec: xcclient.OriginPoolSpec{
			OriginServers: []xcclient.OriginServer{
				{PublicIP: &xcclient.PublicIP{IP: "9.9.9.9"}},
			},
			Port: 8080,
		},
	}

	replaced, err := c.ReplaceOriginPool(ctx, ns, "replace-me", replacement)
	require.NoError(t, err)
	require.NotNil(t, replaced)
	assert.Equal(t, "replace-me", replaced.Metadata.Name)
}

// TestOriginPool_DeleteThenGetReturnsNotFound verifies that deleting a pool
// causes a subsequent Get to return ErrNotFound.
func TestOriginPool_DeleteThenGetReturnsNotFound(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	_, err := c.CreateOriginPool(ctx, ns, sampleOriginPoolCreate("to-delete"))
	require.NoError(t, err)

	err = c.DeleteOriginPool(ctx, ns, "to-delete")
	require.NoError(t, err)

	_, err = c.GetOriginPool(ctx, ns, "to-delete")
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"expected ErrNotFound after delete, got %v", err)
}

// TestOriginPool_ListReturnsCorrectCount verifies that List returns all pools
// in the given namespace.
func TestOriginPool_ListReturnsCorrectCount(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()
	ns := "default"

	for _, name := range []string{"pool-a", "pool-b", "pool-c"} {
		_, err := c.CreateOriginPool(ctx, ns, sampleOriginPoolCreate(name))
		require.NoError(t, err)
	}

	pools, err := c.ListOriginPools(ctx, ns)
	require.NoError(t, err)
	assert.Len(t, pools, 3)
}

// TestOriginPool_GetNonexistentReturnsNotFound verifies that GetOriginPool
// returns ErrNotFound when the pool does not exist.
func TestOriginPool_GetNonexistentReturnsNotFound(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	c := newTestClient(t, srv.URL())
	ctx := context.Background()

	_, err := c.GetOriginPool(ctx, "default", "does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"expected ErrNotFound for missing pool, got %v", err)
}
