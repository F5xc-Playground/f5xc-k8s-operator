package xcclient_test

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/require"
)

// newTestClient builds a *Client pointed at the given server URL. It uses an
// API token so no certificate is needed and registers no Prometheus metrics.
// It is shared by all _test.go files in the xcclient_test package.
func newTestClient(t *testing.T, url string) *xcclient.Client {
	t.Helper()
	cfg := xcclient.Config{
		TenantURL:  url,
		APIToken:   "test-token",
		MaxRetries: 0,
	}
	c, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	return c
}
