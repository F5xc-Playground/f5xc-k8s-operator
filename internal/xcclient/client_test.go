package xcclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_ImplementsXCClient is a compile-time check that *Client satisfies
// the XCClient interface. If *Client is missing any method the file will not
// compile.
func TestClient_ImplementsXCClient(t *testing.T) {
	var _ xcclient.XCClient = (*xcclient.Client)(nil)
}

// validAPITokenConfig returns a minimal valid Config using APIToken auth.
func validAPITokenConfig(serverURL string) xcclient.Config {
	return xcclient.Config{
		TenantURL:  serverURL,
		APIToken:   "test-token-abc123",
		MaxRetries: 3,
	}
}

// TestNewClient_ValidConfig verifies that NewClient succeeds with a valid API
// token configuration.
func TestNewClient_ValidConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// TestNewClient_InvalidConfig verifies that NewClient fails when the config is
// invalid (missing TenantURL).
func TestNewClient_InvalidConfig(t *testing.T) {
	cfg := xcclient.Config{
		// No TenantURL — Validate() will reject this.
		APIToken: "tok",
	}
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.Error(t, err)
	assert.Nil(t, client)
}

// TestAPITokenAuthorizationHeader verifies that the Authorization header is set
// to "APIToken {token}" on every request.
func TestAPITokenAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	// Use the exported Do helper for tests (or we call a resource method if one
	// exists). Since do() is private we use the exported test hook.
	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "default", "my-pool", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "APIToken test-token-abc123", gotAuth)
}

// TestURLConstruction_WithName verifies that a GET with a name produces the
// path /api/config/namespaces/{ns}/{resource}/{name}.
func TestURLConstruction_WithName(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "default", "my-pool", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "/api/config/namespaces/default/origin_pools/my-pool", gotPath)
}

// TestURLConstruction_WithoutName verifies that a GET without a name produces
// the list path /api/config/namespaces/{ns}/{resource}.
func TestURLConstruction_WithoutName(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "default", "", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "/api/config/namespaces/default/origin_pools", gotPath)
}

// TestURLConstruction_IrregularPlural verifies the irregular plural
// service_policys is preserved exactly in the URL path.
func TestURLConstruction_IrregularPlural(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceServicePolicy, "prod", "my-policy", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "/api/config/namespaces/prod/service_policys/my-policy", gotPath)
}

// TestErrorMapping verifies that HTTP status codes are mapped to the correct
// sentinel errors.
func TestErrorMapping(t *testing.T) {
	tests := []struct {
		statusCode int
		sentinel   error
	}{
		{http.StatusNotFound, xcclient.ErrNotFound},
		{http.StatusUnauthorized, xcclient.ErrAuth},
		{http.StatusConflict, xcclient.ErrConflict},
		{http.StatusTooManyRequests, xcclient.ErrRateLimited},
		{http.StatusInternalServerError, xcclient.ErrServerError},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("status_%d", tc.statusCode), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			cfg := validAPITokenConfig(srv.URL)
			// Use MaxRetries=1 (non-zero so Validate leaves it alone) and a
			// tiny base delay so the 429 retry loop completes quickly.
			cfg.MaxRetries = 1
			client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
			require.NoError(t, err)
			client.SetBaseDelay(1 * time.Millisecond)

			doErr := client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "ns", "name", nil, nil)
			require.Error(t, doErr)
			assert.True(t, errors.Is(doErr, tc.sentinel),
				"status %d should map to sentinel %v, got %v", tc.statusCode, tc.sentinel, doErr)
		})
	}
}

// TestRetry_429ThenSuccess verifies that when the server returns 429 twice
// followed by 200, the client succeeds after 3 total attempts.
func TestRetry_429ThenSuccess(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	cfg.MaxRetries = 3
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	client.SetBaseDelay(10 * time.Millisecond) // keep test fast

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "ns", "name", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load(), "expected exactly 3 attempts")
}

// TestRetry_429Exhausted verifies that when the server always returns 429, the
// client gives up after maxRetries and returns ErrRateLimited.
func TestRetry_429Exhausted(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	cfg.MaxRetries = 3
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	client.SetBaseDelay(10 * time.Millisecond)

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "ns", "name", nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrRateLimited),
		"exhausted retries should return ErrRateLimited, got %v", err)
	// maxRetries=3 means: 1 initial attempt + up to 3 retries = 4 total
	assert.Equal(t, int32(4), attempts.Load(), "expected 4 total attempts (1 + 3 retries)")
}

// TestRetry_NonRetryableError verifies that non-429 errors are NOT retried —
// a 404 should result in exactly 1 request.
func TestRetry_NonRetryableError(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	cfg.MaxRetries = 3
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	client.SetBaseDelay(10 * time.Millisecond)

	err = client.Do(context.Background(), http.MethodGet, xcclient.ResourceOriginPool, "ns", "name", nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, xcclient.ErrNotFound))
	assert.Equal(t, int32(1), attempts.Load(), "non-429 errors must not be retried")
}

// TestJSONBodySentOnPost verifies that a body passed to Do is marshalled to
// JSON and sent in the request body with the correct Content-Type header.
func TestJSONBodySentOnPost(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	var gotBody []byte
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		var err error
		gotBody = make([]byte, r.ContentLength)
		_, err = r.Body.Read(gotBody)
		_ = err
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := validAPITokenConfig(srv.URL)
	client, err := xcclient.NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	body := payload{Name: "test-pool", Value: 42}
	err = client.Do(context.Background(), http.MethodPost, xcclient.ResourceOriginPool, "ns", "", body, nil)
	require.NoError(t, err)

	assert.Equal(t, "application/json", gotContentType)

	var decoded payload
	require.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, "test-pool", decoded.Name)
	assert.Equal(t, 42, decoded.Value)
}
