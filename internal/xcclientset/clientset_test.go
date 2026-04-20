package xcclientset_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient satisfies xcclient.XCClient with no-op stub methods.
type fakeClient struct {
	id string
}

// HTTPLoadBalancer
func (f *fakeClient) CreateHTTPLoadBalancer(_ context.Context, _ string, _ *xcclient.HTTPLoadBalancerCreate) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) GetHTTPLoadBalancer(_ context.Context, _, _ string) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceHTTPLoadBalancer(_ context.Context, _, _ string, _ *xcclient.HTTPLoadBalancerReplace) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) DeleteHTTPLoadBalancer(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListHTTPLoadBalancers(_ context.Context, _ string) ([]*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}

// TCPLoadBalancer
func (f *fakeClient) CreateTCPLoadBalancer(_ context.Context, _ string, _ *xcclient.TCPLoadBalancerCreate) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) GetTCPLoadBalancer(_ context.Context, _, _ string) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceTCPLoadBalancer(_ context.Context, _, _ string, _ *xcclient.TCPLoadBalancerReplace) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeClient) DeleteTCPLoadBalancer(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListTCPLoadBalancers(_ context.Context, _ string) ([]*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}

// OriginPool
func (f *fakeClient) CreateOriginPool(_ context.Context, _ string, _ *xcclient.OriginPoolCreate) (*xcclient.OriginPool, error) {
	return nil, nil
}
func (f *fakeClient) GetOriginPool(_ context.Context, _, _ string) (*xcclient.OriginPool, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceOriginPool(_ context.Context, _, _ string, _ *xcclient.OriginPoolReplace) (*xcclient.OriginPool, error) {
	return nil, nil
}
func (f *fakeClient) DeleteOriginPool(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListOriginPools(_ context.Context, _ string) ([]*xcclient.OriginPool, error) {
	return nil, nil
}

// HealthCheck
func (f *fakeClient) CreateHealthCheck(_ context.Context, _ string, _ xcclient.CreateHealthCheck) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeClient) GetHealthCheck(_ context.Context, _, _ string) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceHealthCheck(_ context.Context, _, _ string, _ xcclient.ReplaceHealthCheck) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeClient) DeleteHealthCheck(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListHealthChecks(_ context.Context, _ string) ([]*xcclient.HealthCheck, error) {
	return nil, nil
}

// AppFirewall
func (f *fakeClient) CreateAppFirewall(_ context.Context, _ string, _ *xcclient.AppFirewallCreate) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeClient) GetAppFirewall(_ context.Context, _, _ string) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceAppFirewall(_ context.Context, _, _ string, _ *xcclient.AppFirewallReplace) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeClient) DeleteAppFirewall(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListAppFirewalls(_ context.Context, _ string) ([]*xcclient.AppFirewall, error) {
	return nil, nil
}

// ServicePolicy
func (f *fakeClient) CreateServicePolicy(_ context.Context, _ string, _ *xcclient.ServicePolicyCreate) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeClient) GetServicePolicy(_ context.Context, _, _ string) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceServicePolicy(_ context.Context, _, _ string, _ *xcclient.ServicePolicyReplace) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeClient) DeleteServicePolicy(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListServicePolicies(_ context.Context, _ string) ([]*xcclient.ServicePolicy, error) {
	return nil, nil
}

// XCRateLimiter
func (f *fakeClient) CreateRateLimiter(_ context.Context, _ string, _ xcclient.XCRateLimiterCreate) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeClient) GetRateLimiter(_ context.Context, _, _ string) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeClient) ReplaceRateLimiter(_ context.Context, _, _ string, _ xcclient.XCRateLimiterReplace) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeClient) DeleteRateLimiter(_ context.Context, _, _ string) error { return nil }
func (f *fakeClient) ListRateLimiters(_ context.Context, _ string) ([]*xcclient.XCRateLimiter, error) {
	return nil, nil
}

// Diff helper
func (f *fakeClient) ClientNeedsUpdate(_, _ json.RawMessage) (bool, error) { return false, nil }

// compile-time check
var _ xcclient.XCClient = (*fakeClient)(nil)

func TestClientSet_GetReturnsCurrentClient(t *testing.T) {
	c1 := &fakeClient{id: "first"}
	cs := xcclientset.New(c1)
	assert.Equal(t, c1, cs.Get())
}

func TestClientSet_SwapChangesClient(t *testing.T) {
	c1 := &fakeClient{id: "first"}
	c2 := &fakeClient{id: "second"}
	cs := xcclientset.New(c1)
	cs.Swap(c2)
	got := cs.Get().(*fakeClient)
	assert.Equal(t, "second", got.id)
}

func TestClientSet_ConcurrentAccess(t *testing.T) {
	c1 := &fakeClient{id: "first"}
	c2 := &fakeClient{id: "second"}
	cs := xcclientset.New(c1)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := cs.Get()
			require.NotNil(t, got)
		}()
	}
	cs.Swap(c2)
	wg.Wait()
}
