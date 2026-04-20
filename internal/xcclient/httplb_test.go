package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPLoadBalancer_CreateAndGet verifies that a basic HTTP load balancer
// can be created and retrieved with matching metadata.
func TestHTTPLoadBalancer_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	create := &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "basic-lb", Namespace: "default"},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"app.example.com"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "backend-pool", Namespace: "default"}, Weight: 1},
			},
			HTTP:       json.RawMessage(`{}`),
			RoundRobin: json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateHTTPLoadBalancer(ctx, "default", create)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "basic-lb", created.Metadata.Name)
	assert.Equal(t, "default", created.Metadata.Namespace)

	got, err := client.GetHTTPLoadBalancer(ctx, "default", "basic-lb")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "basic-lb", got.Metadata.Name)
	assert.Equal(t, "default", got.Metadata.Namespace)
	assert.NotEmpty(t, got.RawSpec)
}

// TestHTTPLoadBalancer_CreateWithWAF verifies that an HTTP load balancer can
// be created with an AppFirewall (WAF) reference and that the reference is
// preserved in the retrieved spec.
func TestHTTPLoadBalancer_CreateWithWAF(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	wafRef := &ObjectRef{Name: "my-waf", Namespace: "shared"}
	create := &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "waf-lb", Namespace: "default"},
		Spec: HTTPLoadBalancerSpec{
			Domains:     []string{"secure.example.com"},
			AppFirewall: wafRef,
			HTTP:        json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateHTTPLoadBalancer(ctx, "default", create)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "waf-lb", created.Metadata.Name)

	got, err := client.GetHTTPLoadBalancer(ctx, "default", "waf-lb")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "waf-lb", got.Metadata.Name)
	assert.NotEmpty(t, got.RawSpec)
}

// TestHTTPLoadBalancer_DeleteAndVerifyGone verifies that deleting an HTTP load
// balancer causes a subsequent Get to return ErrNotFound.
func TestHTTPLoadBalancer_DeleteAndVerifyGone(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	_, err := client.CreateHTTPLoadBalancer(ctx, "default", &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "lb-to-delete", Namespace: "default"},
		Spec:     HTTPLoadBalancerSpec{Domains: []string{"delete.example.com"}},
	})
	require.NoError(t, err)

	err = client.DeleteHTTPLoadBalancer(ctx, "default", "lb-to-delete")
	require.NoError(t, err)

	_, err = client.GetHTTPLoadBalancer(ctx, "default", "lb-to-delete")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound, "expected ErrNotFound after delete, got %v", err)
}

// TestHTTPLoadBalancer_ListCount verifies that listing returns all HTTP load
// balancers in a namespace.
func TestHTTPLoadBalancer_ListCount(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	for _, name := range []string{"lb-one", "lb-two", "lb-three"} {
		_, err := client.CreateHTTPLoadBalancer(ctx, "prod", &HTTPLoadBalancerCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "prod"},
			Spec:     HTTPLoadBalancerSpec{Domains: []string{name + ".example.com"}},
		})
		require.NoError(t, err)
	}

	// Create one in a different namespace — must not appear in the list.
	_, err := client.CreateHTTPLoadBalancer(ctx, "staging", &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "lb-other", Namespace: "staging"},
		Spec:     HTTPLoadBalancerSpec{Domains: []string{"other.example.com"}},
	})
	require.NoError(t, err)

	list, err := client.ListHTTPLoadBalancers(ctx, "prod")
	require.NoError(t, err)
	assert.Len(t, list, 3, "expected exactly 3 HTTP load balancers in namespace prod")
}
