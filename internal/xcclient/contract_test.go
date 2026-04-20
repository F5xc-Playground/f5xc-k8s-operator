//go:build contract

package xcclient

import (
	"context"
	"errors"
	"os"
	"testing"

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
