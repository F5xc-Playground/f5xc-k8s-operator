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

// TestHealthCheck_CreateAndGet verifies that a HealthCheck created with an
// HTTP health check spec can be retrieved with the same spec values.
func TestHealthCheck_CreateAndGet(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	client := newTestClient(t, srv.URL())
	ctx := context.Background()

	spec := xcclient.HealthCheckSpec{
		HTTPHealthCheck: &xcclient.HTTPHealthCheck{
			Path:                "/healthz",
			UseHTTP2:            false,
			ExpectedStatusCodes: []string{"200", "204"},
		},
		HealthyThreshold:   3,
		UnhealthyThreshold: 2,
		Interval:           15,
		Timeout:            5,
		JitterPercent:      10,
	}

	// Create the health check.
	created, err := client.CreateHealthCheck(ctx, "default", xcclient.CreateHealthCheck{
		Metadata: xcclient.ObjectMeta{Name: "my-hc", Namespace: "default"},
		Spec:     spec,
	})
	require.NoError(t, err)
	assert.Equal(t, "my-hc", created.Metadata.Name)
	assert.Equal(t, "default", created.Metadata.Namespace)

	// Retrieve and round-trip the spec.
	got, err := client.GetHealthCheck(ctx, "default", "my-hc")
	require.NoError(t, err)
	assert.Equal(t, "my-hc", got.Metadata.Name)

	gotSpec, err := got.ParseSpec()
	require.NoError(t, err)
	require.NotNil(t, gotSpec.HTTPHealthCheck)
	assert.Equal(t, "/healthz", gotSpec.HTTPHealthCheck.Path)
	assert.Equal(t, []string{"200", "204"}, gotSpec.HTTPHealthCheck.ExpectedStatusCodes)
	assert.Equal(t, uint32(3), gotSpec.HealthyThreshold)
	assert.Equal(t, uint32(2), gotSpec.UnhealthyThreshold)
	assert.Equal(t, uint32(15), gotSpec.Interval)
	assert.Equal(t, uint32(5), gotSpec.Timeout)
	assert.Equal(t, uint32(10), gotSpec.JitterPercent)
}

// TestHealthCheck_DeleteAndVerifyGone verifies that after deleting a
// HealthCheck a subsequent Get returns ErrNotFound.
func TestHealthCheck_DeleteAndVerifyGone(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	client := newTestClient(t, srv.URL())
	ctx := context.Background()

	// Create first.
	_, err := client.CreateHealthCheck(ctx, "default", xcclient.CreateHealthCheck{
		Metadata: xcclient.ObjectMeta{Name: "hc-to-delete", Namespace: "default"},
		Spec: xcclient.HealthCheckSpec{
			TCPHealthCheck: &xcclient.TCPHealthCheck{
				Send:    "PING",
				Receive: "PONG",
			},
			Interval: 10,
			Timeout:  3,
		},
	})
	require.NoError(t, err)

	// Delete it.
	err = client.DeleteHealthCheck(ctx, "default", "hc-to-delete")
	require.NoError(t, err)

	// Now Get should return ErrNotFound.
	_, err = client.GetHealthCheck(ctx, "default", "hc-to-delete")
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrNotFound),
		"expected ErrNotFound after delete, got %v", err)
}

// TestHealthCheck_ListCount verifies that listing returns the correct number
// of health checks in a namespace.
func TestHealthCheck_ListCount(t *testing.T) {
	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	client := newTestClient(t, srv.URL())
	ctx := context.Background()

	// Create three health checks.
	names := []string{"hc-one", "hc-two", "hc-three"}
	for _, name := range names {
		_, err := client.CreateHealthCheck(ctx, "prod", xcclient.CreateHealthCheck{
			Metadata: xcclient.ObjectMeta{Name: name, Namespace: "prod"},
			Spec: xcclient.HealthCheckSpec{
				HTTPHealthCheck: &xcclient.HTTPHealthCheck{
					Path: "/ping",
				},
				Interval: 30,
				Timeout:  10,
			},
		})
		require.NoError(t, err)
	}

	// Also create one in a different namespace — must not appear in the list.
	_, err := client.CreateHealthCheck(ctx, "staging", xcclient.CreateHealthCheck{
		Metadata: xcclient.ObjectMeta{Name: "hc-other", Namespace: "staging"},
		Spec:     xcclient.HealthCheckSpec{Interval: 30, Timeout: 10},
	})
	require.NoError(t, err)

	list, err := client.ListHealthChecks(ctx, "prod")
	require.NoError(t, err)
	assert.Len(t, list, 3, "expected exactly 3 health checks in namespace prod")
}
