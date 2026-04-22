//go:build contract

package xcclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contractClient reads XC_TENANT_URL and XC_API_TOKEN from the environment,
// skips the test if either is absent, and returns a real *Client configured
// to log via t.Logf.
func contractClient(t *testing.T) *Client {
	t.Helper()

	tenantURL := os.Getenv("XC_TENANT_URL")
	apiToken := os.Getenv("XC_API_TOKEN")

	if tenantURL == "" || apiToken == "" {
		t.Skip("skipping contract test: XC_TENANT_URL and XC_API_TOKEN must be set")
	}

	log := funcr.New(func(prefix, args string) {
		t.Logf("%s: %s", prefix, args)
	}, funcr.Options{Verbosity: 1})

	cfg := Config{
		TenantURL: tenantURL,
		APIToken:  apiToken,
	}

	c, err := NewClient(cfg, log, nil)
	require.NoError(t, err, "creating contract client")
	return c
}

// contractNamespace returns the namespace to use for contract tests.
// It reads XC_TEST_NAMESPACE from the environment and defaults to
// "operator-test".
func contractNamespace(t *testing.T) string {
	t.Helper()
	ns := os.Getenv("XC_TEST_NAMESPACE")
	if ns == "" {
		return "operator-test"
	}
	return ns
}

// TestContract_OriginPool_CRUD exercises the full create/get/replace/list/delete
// lifecycle of an origin pool against a real F5 XC API.
func TestContract_OriginPool_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-origin-pool"

	// Cleanup from any prior run — ignore the error if it doesn't exist.
	_ = c.DeleteOriginPool(ctx, ns, name)

	// --- Create ---
	created, err := c.CreateOriginPool(ctx, ns, &OriginPoolCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: OriginPoolSpec{
			OriginServers: []OriginServer{
				{PublicIP: &PublicIP{IP: "1.2.3.4"}},
			},
			Port:                  443,
			LoadBalancerAlgorithm: "LB_OVERRIDE_ALGO_ROUND_ROBIN",
		},
	})
	require.NoError(t, err, "CreateOriginPool")
	require.NotNil(t, created)
	assert.Equal(t, name, created.Metadata.Name, "created name")
	assert.NotEmpty(t, created.SystemMetadata.UID, "system metadata UID should be set after create")

	// --- Get ---
	got, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err, "GetOriginPool after create")
	require.NotNil(t, got)
	assert.NotEmpty(t, got.RawSpec, "RawSpec should be non-empty after get")

	resourceVersion := got.Metadata.ResourceVersion

	// --- Replace ---
	_, err = c.ReplaceOriginPool(ctx, ns, name, &OriginPoolReplace{
		Metadata: ObjectMeta{
			Name:            name,
			ResourceVersion: resourceVersion,
		},
		Spec: OriginPoolSpec{
			OriginServers: []OriginServer{
				{PublicIP: &PublicIP{IP: "5.6.7.8"}},
			},
			Port:                  8080,
			LoadBalancerAlgorithm: "LB_OVERRIDE_ALGO_ROUND_ROBIN",
		},
	})
	require.NoError(t, err, "ReplaceOriginPool")

	// --- Get after replace ---
	updated, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err, "GetOriginPool after replace")
	require.NotNil(t, updated)
	assert.Equal(t, 8080, updated.Spec.Port, "port should reflect replacement")

	// --- List ---
	pools, err := c.ListOriginPools(ctx, ns)
	require.NoError(t, err, "ListOriginPools")
	found := false
	for _, p := range pools {
		if p.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "pool should appear in list")

	// --- Delete ---
	err = c.DeleteOriginPool(ctx, ns, name)
	require.NoError(t, err, "DeleteOriginPool")

	// --- Get after delete —should return ErrNotFound ---
	_, err = c.GetOriginPool(ctx, ns, name)
	require.Error(t, err, "GetOriginPool after delete should error")
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound after delete, got %v", err)
}

// TestContract_HealthCheck_CRUD exercises the full create/get/delete lifecycle
// of an HTTP health check against a real F5 XC API.
func TestContract_HealthCheck_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-healthcheck"

	// Cleanup from any prior run.
	_ = c.DeleteHealthCheck(ctx, ns, name)

	// --- Create ---
	created, err := c.CreateHealthCheck(ctx, ns, CreateHealthCheck{
		Metadata: ObjectMeta{Name: name},
		Spec: HealthCheckSpec{
			HTTPHealthCheck: &HTTPHealthCheck{
				Path: "/healthz",
			},
			HealthyThreshold:   3,
			UnhealthyThreshold: 3,
			Interval:           15,
			Timeout:            5,
		},
	})
	require.NoError(t, err, "CreateHealthCheck")
	require.NotNil(t, created)
	assert.Equal(t, name, created.Metadata.Name, "created name")

	// --- Get ---
	got, err := c.GetHealthCheck(ctx, ns, name)
	require.NoError(t, err, "GetHealthCheck after create")
	require.NotNil(t, got)
	assert.Equal(t, name, got.Metadata.Name, "got name")

	// --- Delete ---
	err = c.DeleteHealthCheck(ctx, ns, name)
	require.NoError(t, err, "DeleteHealthCheck")

	// --- Get after delete — should return ErrNotFound ---
	_, err = c.GetHealthCheck(ctx, ns, name)
	require.Error(t, err, "GetHealthCheck after delete should error")
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound after delete, got %v", err)
}

// TestContract_AppFirewall_CRUD exercises the full create/get/delete lifecycle
// of an App Firewall (using monitoring-mode defaults) against a real F5 XC API.
func TestContract_AppFirewall_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-appfirewall"

	// Cleanup from any prior run.
	_ = c.DeleteAppFirewall(ctx, ns, name)

	// --- Create with monitoring mode defaults ---
	created, err := c.CreateAppFirewall(ctx, ns, &AppFirewallCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: AppFirewallSpec{
			DefaultDetectionSettings: []byte(`{}`),
			Monitoring:               []byte(`{}`),
			UseDefaultBlockingPage:   []byte(`{}`),
			AllowAllResponseCodes:    []byte(`{}`),
			DefaultBotSetting:        []byte(`{}`),
			DefaultAnonymization:     []byte(`{}`),
		},
	})
	require.NoError(t, err, "CreateAppFirewall")
	require.NotNil(t, created)
	assert.Equal(t, name, created.Metadata.Name, "created name")

	// --- Get ---
	got, err := c.GetAppFirewall(ctx, ns, name)
	require.NoError(t, err, "GetAppFirewall after create")
	require.NotNil(t, got)
	assert.Equal(t, name, got.Metadata.Name, "got name")

	// --- Delete ---
	err = c.DeleteAppFirewall(ctx, ns, name)
	require.NoError(t, err, "DeleteAppFirewall")

	// --- Get after delete — should return ErrNotFound ---
	_, err = c.GetAppFirewall(ctx, ns, name)
	require.Error(t, err, "GetAppFirewall after delete should error")
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound after delete, got %v", err)
}

// ---------------------------------------------------------------------------
// OriginPool — field variations
// ---------------------------------------------------------------------------

func TestContract_OriginPool_PublicName(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-op-pubname"
	_ = c.DeleteOriginPool(ctx, ns, name)

	created, err := c.CreateOriginPool(ctx, ns, &OriginPoolCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: OriginPoolSpec{
			OriginServers: []OriginServer{
				{PublicName: &PublicName{DNSName: "example.com"}},
			},
			Port: 443,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err)
	require.Len(t, got.Spec.OriginServers, 1)
	require.NotNil(t, got.Spec.OriginServers[0].PublicName)
	assert.Equal(t, "example.com", got.Spec.OriginServers[0].PublicName.DNSName)

	require.NoError(t, c.DeleteOriginPool(ctx, ns, name))
}

func TestContract_OriginPool_MultipleOriginsWithAlgorithm(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-op-multi"
	_ = c.DeleteOriginPool(ctx, ns, name)

	created, err := c.CreateOriginPool(ctx, ns, &OriginPoolCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: OriginPoolSpec{
			OriginServers: []OriginServer{
				{PublicIP: &PublicIP{IP: "1.2.3.4"}},
				{PublicName: &PublicName{DNSName: "backend.example.com"}},
				{PublicIP: &PublicIP{IP: "5.6.7.8"}},
			},
			Port:                  8080,
			LoadBalancerAlgorithm: "LB_OVERRIDE_ALGO_LEAST_ACTIVE",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err)
	assert.Len(t, got.Spec.OriginServers, 3)
	assert.Equal(t, 8080, got.Spec.Port)

	require.NoError(t, c.DeleteOriginPool(ctx, ns, name))
}

func TestContract_OriginPool_NoTLS(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-op-notls"
	_ = c.DeleteOriginPool(ctx, ns, name)

	_, err := c.CreateOriginPool(ctx, ns, &OriginPoolCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: OriginPoolSpec{
			OriginServers: []OriginServer{
				{PublicIP: &PublicIP{IP: "10.0.0.1"}},
			},
			Port:  80,
			NoTLS: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	got, err := c.GetOriginPool(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, 80, got.Spec.Port)

	require.NoError(t, c.DeleteOriginPool(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// HealthCheck — field variations
// ---------------------------------------------------------------------------

func TestContract_HealthCheck_TCP(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-hc-tcp"
	_ = c.DeleteHealthCheck(ctx, ns, name)

	created, err := c.CreateHealthCheck(ctx, ns, CreateHealthCheck{
		Metadata: ObjectMeta{Name: name},
		Spec: HealthCheckSpec{
			TCPHealthCheck: &TCPHealthCheck{
				SendPayload:      "PING",
				ExpectedResponse: "PONG",
			},
			Interval: 10,
			Timeout:  3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetHealthCheck(ctx, ns, name)
	require.NoError(t, err)
	spec, err := got.ParseSpec()
	require.NoError(t, err)
	require.NotNil(t, spec.TCPHealthCheck)
	assert.Equal(t, "PING", spec.TCPHealthCheck.SendPayload)
	assert.Equal(t, "PONG", spec.TCPHealthCheck.ExpectedResponse)

	require.NoError(t, c.DeleteHealthCheck(ctx, ns, name))
}

func TestContract_HealthCheck_HTTPWithThresholds(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-hc-thresholds"
	_ = c.DeleteHealthCheck(ctx, ns, name)

	created, err := c.CreateHealthCheck(ctx, ns, CreateHealthCheck{
		Metadata: ObjectMeta{Name: name},
		Spec: HealthCheckSpec{
			HTTPHealthCheck: &HTTPHealthCheck{
				Path: "/ready",
			},
			HealthyThreshold:   5,
			UnhealthyThreshold: 3,
			Interval:           30,
			Timeout:            10,
			JitterPercent:      20,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetHealthCheck(ctx, ns, name)
	require.NoError(t, err)
	spec, err := got.ParseSpec()
	require.NoError(t, err)
	assert.Equal(t, uint32(5), spec.HealthyThreshold)
	assert.Equal(t, uint32(3), spec.UnhealthyThreshold)
	assert.Equal(t, uint32(30), spec.Interval)
	assert.Equal(t, uint32(10), spec.Timeout)
	assert.Equal(t, uint32(20), spec.JitterPercent)

	require.NoError(t, c.DeleteHealthCheck(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// AppFirewall — blocking mode variation
// ---------------------------------------------------------------------------

func TestContract_AppFirewall_BlockingMode(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-afw-blocking"
	_ = c.DeleteAppFirewall(ctx, ns, name)

	created, err := c.CreateAppFirewall(ctx, ns, &AppFirewallCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: AppFirewallSpec{
			DefaultDetectionSettings: json.RawMessage(`{}`),
			Blocking:                 json.RawMessage(`{}`),
			UseDefaultBlockingPage:   json.RawMessage(`{}`),
			AllowAllResponseCodes:    json.RawMessage(`{}`),
			DefaultBotSetting:        json.RawMessage(`{}`),
			DisableAnonymization:     json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetAppFirewall(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.Blocking)
	assert.NotNil(t, got.Spec.DisableAnonymization)

	require.NoError(t, c.DeleteAppFirewall(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// ServicePolicy — full CRUD and variations
// ---------------------------------------------------------------------------

func TestContract_ServicePolicy_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-sp"
	_ = c.DeleteServicePolicy(ctx, ns, name)

	created, err := c.CreateServicePolicy(ctx, ns, &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: ServicePolicySpec{
			AllowAllRequests: json.RawMessage(`{}`),
			AnyServer:        json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)
	assert.NotEmpty(t, created.SystemMetadata.UID)

	got, err := c.GetServicePolicy(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.AllowAllRequests)
	assert.NotNil(t, got.Spec.AnyServer)

	resourceVersion := got.Metadata.ResourceVersion

	_, err = c.ReplaceServicePolicy(ctx, ns, name, &ServicePolicyReplace{
		Metadata: ObjectMeta{Name: name, ResourceVersion: resourceVersion},
		Spec: ServicePolicySpec{
			DenyAllRequests: json.RawMessage(`{}`),
			AnyServer:       json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	updated, err := c.GetServicePolicy(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, updated.Spec.DenyAllRequests)

	policies, err := c.ListServicePolicies(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, p := range policies {
		if p.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "policy should appear in list")

	require.NoError(t, c.DeleteServicePolicy(ctx, ns, name))
	_, err = c.GetServicePolicy(ctx, ns, name)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestContract_ServicePolicy_DenyAllWithServerName(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-sp-servername"
	_ = c.DeleteServicePolicy(ctx, ns, name)

	_, err := c.CreateServicePolicy(ctx, ns, &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: ServicePolicySpec{
			DenyAllRequests: json.RawMessage(`{}`),
			ServerName:      "api.example.com",
		},
	})
	require.NoError(t, err)

	got, err := c.GetServicePolicy(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.DenyAllRequests)
	assert.Equal(t, "api.example.com", got.Spec.ServerName)

	require.NoError(t, c.DeleteServicePolicy(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// RateLimiter — full CRUD and variations
// ---------------------------------------------------------------------------

func TestContract_RateLimiter_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-rl"
	_ = c.DeleteRateLimiter(ctx, ns, name)

	created, err := c.CreateRateLimiter(ctx, ns, XCRateLimiterCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: XCRateLimiterSpec{
			Limits: []RateLimitValue{{TotalNumber: 100, Unit: "MINUTE"}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)
	assert.NotEmpty(t, created.SystemMetadata.UID)

	got, err := c.GetRateLimiter(ctx, ns, name)
	require.NoError(t, err)
	require.Len(t, got.Spec.Limits, 1)
	assert.Equal(t, uint32(100), got.Spec.Limits[0].TotalNumber)
	assert.Equal(t, "MINUTE", got.Spec.Limits[0].Unit)

	resourceVersion := got.Metadata.ResourceVersion
	_, err = c.ReplaceRateLimiter(ctx, ns, name, XCRateLimiterReplace{
		Metadata: ObjectMeta{Name: name, ResourceVersion: resourceVersion},
		Spec: XCRateLimiterSpec{
			Limits: []RateLimitValue{{TotalNumber: 200, Unit: "SECOND"}},
		},
	})
	require.NoError(t, err)

	updated, err := c.GetRateLimiter(ctx, ns, name)
	require.NoError(t, err)
	require.Len(t, updated.Spec.Limits, 1)
	assert.Equal(t, uint32(200), updated.Spec.Limits[0].TotalNumber)
	assert.Equal(t, "SECOND", updated.Spec.Limits[0].Unit)

	limiters, err := c.ListRateLimiters(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, rl := range limiters {
		if rl.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "rate limiter should appear in list")

	require.NoError(t, c.DeleteRateLimiter(ctx, ns, name))
	_, err = c.GetRateLimiter(ctx, ns, name)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestContract_RateLimiter_WithBurst(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-rl-burst"
	_ = c.DeleteRateLimiter(ctx, ns, name)

	_, err := c.CreateRateLimiter(ctx, ns, XCRateLimiterCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: XCRateLimiterSpec{
			Limits: []RateLimitValue{{TotalNumber: 50, Unit: "SECOND", BurstMultiplier: 3}},
		},
	})
	require.NoError(t, err)

	got, err := c.GetRateLimiter(ctx, ns, name)
	require.NoError(t, err)
	require.Len(t, got.Spec.Limits, 1)
	assert.Equal(t, uint32(50), got.Spec.Limits[0].TotalNumber)
	assert.Equal(t, uint32(3), got.Spec.Limits[0].BurstMultiplier)

	require.NoError(t, c.DeleteRateLimiter(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// TCPLoadBalancer — full CRUD and variations
// ---------------------------------------------------------------------------

func TestContract_TCPLoadBalancer_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-tlb"
	_ = c.DeleteTCPLoadBalancer(ctx, ns, name)

	created, err := c.CreateTCPLoadBalancer(ctx, ns, &TCPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: TCPLoadBalancerSpec{
			Domains:    []string{"tcp.contract.test"},
			ListenPort: 9090,
			OriginPoolWeights: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetTCPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, []string{"tcp.contract.test"}, got.Spec.Domains)
	assert.Equal(t, uint32(9090), got.Spec.ListenPort)
	require.Len(t, got.Spec.OriginPoolWeights, 1)

	resourceVersion := got.Metadata.ResourceVersion
	_, err = c.ReplaceTCPLoadBalancer(ctx, ns, name, &TCPLoadBalancerReplace{
		Metadata: ObjectMeta{Name: name, ResourceVersion: resourceVersion},
		Spec: TCPLoadBalancerSpec{
			Domains:    []string{"tcp2.contract.test"},
			ListenPort: 8443,
			OriginPoolWeights: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	updated, err := c.GetTCPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, uint32(8443), updated.Spec.ListenPort)

	lbs, err := c.ListTCPLoadBalancers(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, lb := range lbs {
		if lb.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "TCP LB should appear in list")

	require.NoError(t, c.DeleteTCPLoadBalancer(ctx, ns, name))
	_, err = c.GetTCPLoadBalancer(ctx, ns, name)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestContract_TCPLoadBalancer_DoNotAdvertise(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-tlb-noadv"
	_ = c.DeleteTCPLoadBalancer(ctx, ns, name)

	_, err := c.CreateTCPLoadBalancer(ctx, ns, &TCPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: TCPLoadBalancerSpec{
			Domains:    []string{"internal.contract.test"},
			ListenPort: 8080,
			OriginPoolWeights: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			DoNotAdvertise: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	got, err := c.GetTCPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.DoNotAdvertise)

	require.NoError(t, c.DeleteTCPLoadBalancer(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// HTTPLoadBalancer — full CRUD and variations
// ---------------------------------------------------------------------------

func TestContract_HTTPLoadBalancer_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-hlb"
	_ = c.DeleteHTTPLoadBalancer(ctx, ns, name)

	created, err := c.CreateHTTPLoadBalancer(ctx, ns, &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"http.contract.test"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			HTTP:                       json.RawMessage(`{"dns_volterra_managed":true}`),
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)
	assert.NotEmpty(t, created.SystemMetadata.UID)

	got, err := c.GetHTTPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, []string{"http.contract.test"}, got.Spec.Domains)
	require.Len(t, got.Spec.DefaultRoutePools, 1)

	resourceVersion := got.Metadata.ResourceVersion
	_, err = c.ReplaceHTTPLoadBalancer(ctx, ns, name, &HTTPLoadBalancerReplace{
		Metadata: ObjectMeta{Name: name, ResourceVersion: resourceVersion},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"http2.contract.test"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			HTTP:                       json.RawMessage(`{"dns_volterra_managed":true}`),
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	updated, err := c.GetHTTPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, []string{"http2.contract.test"}, updated.Spec.Domains)

	lbs, err := c.ListHTTPLoadBalancers(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, lb := range lbs {
		if lb.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "HTTP LB should appear in list")

	require.NoError(t, c.DeleteHTTPLoadBalancer(ctx, ns, name))
	_, err = c.GetHTTPLoadBalancer(ctx, ns, name)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestContract_HTTPLoadBalancer_HTTPSAutoCert(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-hlb-autocert"
	_ = c.DeleteHTTPLoadBalancer(ctx, ns, name)

	_, err := c.CreateHTTPLoadBalancer(ctx, ns, &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"autocert.contract.test"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			HTTPSAutoCert:              json.RawMessage(`{"add_hsts":true,"http_redirect":true,"no_mtls":{}}`),
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	got, err := c.GetHTTPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.HTTPSAutoCert)

	require.NoError(t, c.DeleteHTTPLoadBalancer(ctx, ns, name))
}

func TestContract_HTTPLoadBalancer_WithDisableOptions(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-hlb-disabled"
	_ = c.DeleteHTTPLoadBalancer(ctx, ns, name)

	_, err := c.CreateHTTPLoadBalancer(ctx, ns, &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"disabled.contract.test"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "contract-test-origin-pool", Namespace: ns}, Weight: 1},
			},
			HTTP:                       json.RawMessage(`{"dns_volterra_managed":true}`),
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
			DisableWAF:                 json.RawMessage(`{}`),
			DisableBotDefense:          json.RawMessage(`{}`),
			DisableAPIDiscovery:        json.RawMessage(`{}`),
			DisableIPReputation:        json.RawMessage(`{}`),
			DisableRateLimit:           json.RawMessage(`{}`),
			NoChallenge:                json.RawMessage(`{}`),
			NoServicePolicies:          json.RawMessage(`{}`),
			RoundRobin:                 json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)

	got, err := c.GetHTTPLoadBalancer(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, got.Spec.DisableWAF)
	assert.NotNil(t, got.Spec.DisableBotDefense)
	assert.NotNil(t, got.Spec.NoChallenge)

	require.NoError(t, c.DeleteHTTPLoadBalancer(ctx, ns, name))
}

// ---------------------------------------------------------------------------
// Certificate — full CRUD
// ---------------------------------------------------------------------------

func generateTestCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "contract-test.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

func TestContract_Certificate_CRUD(t *testing.T) {
	c := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()

	const name = "contract-test-cert"
	_ = c.DeleteCertificate(ctx, ns, name)

	certPEM, keyPEM := generateTestCertPEM(t)
	certURL := fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(certPEM))
	keyURL := fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(keyPEM))

	created, err := c.CreateCertificate(ctx, ns, &CertificateCreate{
		Metadata: ObjectMeta{Name: name},
		Spec: CertificateSpec{
			CertificateURL: certURL,
			PrivateKey: CertificatePrivKey{
				ClearSecretInfo: &ClearSecretInfo{
					URL: keyURL,
				},
			},
			DisableOcspStapling: json.RawMessage(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := c.GetCertificate(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Metadata.Name)
	assert.NotEmpty(t, got.Spec.CertificateURL)

	resourceVersion := got.Metadata.ResourceVersion

	newCertPEM, newKeyPEM := generateTestCertPEM(t)
	newCertURL := fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(newCertPEM))
	newKeyURL := fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(newKeyPEM))

	_, err = c.ReplaceCertificate(ctx, ns, name, &CertificateReplace{
		Metadata: ObjectMeta{Name: name, ResourceVersion: resourceVersion},
		Spec: CertificateSpec{
			CertificateURL: newCertURL,
			PrivateKey: CertificatePrivKey{
				ClearSecretInfo: &ClearSecretInfo{
					URL: newKeyURL,
				},
			},
		},
	})
	require.NoError(t, err)

	certs, err := c.ListCertificates(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, cert := range certs {
		if cert.Metadata.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "certificate should appear in list")

	require.NoError(t, c.DeleteCertificate(ctx, ns, name))
	_, err = c.GetCertificate(ctx, ns, name)
	assert.True(t, errors.Is(err, ErrNotFound))
}
