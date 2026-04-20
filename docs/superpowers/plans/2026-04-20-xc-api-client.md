# F5 XC API Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go client library for the F5 Distributed Cloud REST API covering 7 resource types with dual auth, per-endpoint rate limiting, change detection, and retry logic.

**Architecture:** Hand-rolled Go client at `internal/xcclient/` with a single `XCClient` interface. Each resource gets its own file with typed structs and thin CRUD methods that delegate to a shared `do()` helper. Unknown fields are preserved via `json.RawMessage` overlay on PUT. Tests use `httptest` fake server and build-tag-gated contract tests.

**Tech Stack:** Go 1.22+, `golang.org/x/time/rate`, `github.com/prometheus/client_golang`, `github.com/go-logr/logr`, `github.com/stretchr/testify`

---

## File Map

| File | Responsibility |
|------|---------------|
| `go.mod` | Module definition |
| `Makefile` | Build, test, lint targets |
| `internal/xcclient/errors.go` | Sentinel error types + wrapping |
| `internal/xcclient/errors_test.go` | Error wrapping/unwrapping tests |
| `internal/xcclient/config.go` | Config struct, defaults, validation |
| `internal/xcclient/config_test.go` | Config validation tests |
| `internal/xcclient/types.go` | ObjectMeta, SystemMeta, shared JSON types |
| `internal/xcclient/types_test.go` | JSON round-trip tests for shared types |
| `internal/xcclient/ratelimit.go` | Per-endpoint token bucket rate limiter |
| `internal/xcclient/ratelimit_test.go` | Rate limiter isolation + burst tests |
| `internal/xcclient/metrics.go` | Prometheus metric definitions + recording helpers |
| `internal/xcclient/client.go` | Client struct, constructor, `do()` helper, auth, retry |
| `internal/xcclient/client_test.go` | Transport, auth header, retry, error mapping tests |
| `internal/xcclient/diff.go` | `NeedsUpdate` change detection |
| `internal/xcclient/diff_test.go` | Change detection tests |
| `internal/xcclient/testutil/fakeserver.go` | Reusable httptest fake XC API |
| `internal/xcclient/originpool.go` | OriginPool types + CRUD methods |
| `internal/xcclient/originpool_test.go` | OriginPool unit + integration tests |
| `internal/xcclient/healthcheck.go` | HealthCheck types + CRUD methods |
| `internal/xcclient/healthcheck_test.go` | HealthCheck tests |
| `internal/xcclient/appfirewall.go` | AppFirewall types + CRUD methods |
| `internal/xcclient/appfirewall_test.go` | AppFirewall tests |
| `internal/xcclient/httplb.go` | HTTPLoadBalancer types + CRUD methods |
| `internal/xcclient/httplb_test.go` | HTTPLoadBalancer tests |
| `internal/xcclient/tcplb.go` | TCPLoadBalancer types + CRUD methods |
| `internal/xcclient/tcplb_test.go` | TCPLoadBalancer tests |
| `internal/xcclient/servicepolicy.go` | ServicePolicy types + CRUD methods |
| `internal/xcclient/servicepolicy_test.go` | ServicePolicy tests |
| `internal/xcclient/xcratelimiter.go` | RateLimiter (XC resource) types + CRUD methods |
| `internal/xcclient/xcratelimiter_test.go` | RateLimiter (XC resource) tests |
| `internal/xcclient/interface.go` | XCClient interface definition |
| `internal/xcclient/contract_test.go` | Contract tests against real XC API |

---

### Task 1: Go Module + Makefile

**Files:**
- Create: `go.mod`
- Create: `Makefile`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go mod init github.com/kreynolds/f5xc-k8s-operator
```

Expected output: `go: creating new go.mod: module github.com/kreynolds/f5xc-k8s-operator`

- [ ] **Step 2: Add dependencies**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go get golang.org/x/time/rate
go get github.com/prometheus/client_golang/prometheus
go get github.com/go-logr/logr
go get github.com/stretchr/testify
go get software.sslmate.com/src/go-pkcs12
```

- [ ] **Step 3: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: test test-contract lint fmt vet

test:
	go test ./... -v -count=1

test-contract:
	go test ./... -v -count=1 -tags=contract

lint: fmt vet
	@echo "Lint passed"

fmt:
	gofmt -s -w .

vet:
	go vet ./...
```

- [ ] **Step 4: Create internal directory structure**

```bash
mkdir -p internal/xcclient/testutil
```

- [ ] **Step 5: Verify module builds**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go build ./...
```

Expected: no output (clean build with no source files yet is fine).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum Makefile
git commit -m "Initialize Go module with dependencies and Makefile"
```

---

### Task 2: Error Types

**Files:**
- Create: `internal/xcclient/errors.go`
- Create: `internal/xcclient/errors_test.go`

- [ ] **Step 1: Write failing tests for error types**

Create `internal/xcclient/errors_test.go`:

```go
package xcclient

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrNotFound)
	assert.Error(t, ErrConflict)
	assert.Error(t, ErrRateLimited)
	assert.Error(t, ErrServerError)
	assert.Error(t, ErrAuth)
}

func TestAPIError_Wraps_Sentinel(t *testing.T) {
	err := &APIError{
		StatusCode: 404,
		Endpoint:   "origin_pools",
		Message:    "object not found",
		Err:        ErrNotFound,
	}

	assert.True(t, errors.Is(err, ErrNotFound))
	assert.False(t, errors.Is(err, ErrConflict))
}

func TestAPIError_Unwrap(t *testing.T) {
	inner := ErrRateLimited
	err := &APIError{
		StatusCode: 429,
		Endpoint:   "http_loadbalancers",
		Message:    "rate limited",
		Err:        inner,
	}

	var target *APIError
	require.True(t, errors.As(err, &target))
	assert.Equal(t, 429, target.StatusCode)
	assert.Equal(t, "http_loadbalancers", target.Endpoint)
}

func TestAPIError_ErrorMessage(t *testing.T) {
	err := &APIError{
		StatusCode: 500,
		Endpoint:   "origin_pools",
		Message:    "internal error",
		Err:        ErrServerError,
	}

	msg := err.Error()
	assert.Contains(t, msg, "500")
	assert.Contains(t, msg, "origin_pools")
	assert.Contains(t, msg, "internal error")
}

func TestStatusToError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		endpoint   string
		body       string
		wantErr    error
	}{
		{"404 maps to ErrNotFound", 404, "origin_pools", "not found", ErrNotFound},
		{"401 maps to ErrAuth", 401, "origin_pools", "unauthorized", ErrAuth},
		{"403 maps to ErrAuth", 403, "origin_pools", "forbidden", ErrAuth},
		{"409 maps to ErrConflict", 409, "origin_pools", "conflict", ErrConflict},
		{"429 maps to ErrRateLimited", 429, "origin_pools", "rate limited", ErrRateLimited},
		{"500 maps to ErrServerError", 500, "origin_pools", "server error", ErrServerError},
		{"502 maps to ErrServerError", 502, "origin_pools", "bad gateway", ErrServerError},
		{"503 maps to ErrServerError", 503, "origin_pools", "unavailable", ErrServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StatusToError(tt.statusCode, tt.endpoint, tt.body)
			assert.True(t, errors.Is(err, tt.wantErr))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestSentinel
```

Expected: FAIL — package doesn't compile.

- [ ] **Step 3: Implement error types**

Create `internal/xcclient/errors.go`:

```go
package xcclient

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound    = errors.New("xc: resource not found")
	ErrConflict    = errors.New("xc: resource conflict")
	ErrRateLimited = errors.New("xc: rate limited")
	ErrServerError = errors.New("xc: server error")
	ErrAuth        = errors.New("xc: authentication failed")
)

type APIError struct {
	StatusCode int
	Endpoint   string
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("xc api %s: status %d: %s", e.Endpoint, e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func StatusToError(statusCode int, endpoint, body string) error {
	var sentinel error
	switch {
	case statusCode == 404:
		sentinel = ErrNotFound
	case statusCode == 401 || statusCode == 403:
		sentinel = ErrAuth
	case statusCode == 409:
		sentinel = ErrConflict
	case statusCode == 429:
		sentinel = ErrRateLimited
	case statusCode >= 500:
		sentinel = ErrServerError
	default:
		sentinel = fmt.Errorf("xc: unexpected status %d", statusCode)
	}

	return &APIError{
		StatusCode: statusCode,
		Endpoint:   endpoint,
		Message:    body,
		Err:        sentinel,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run "TestSentinel|TestAPIError|TestStatusToError"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/errors.go internal/xcclient/errors_test.go
git commit -m "Add typed error types with sentinel wrapping and status code mapping"
```

---

### Task 3: Config + Validation

**Files:**
- Create: `internal/xcclient/config.go`
- Create: `internal/xcclient/config_test.go`

- [ ] **Step 1: Write failing tests for config validation**

Create `internal/xcclient/config_test.go`:

```go
package xcclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_RequiresTenantURL(t *testing.T) {
	cfg := Config{
		APIToken: "test-token",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TenantURL")
}

func TestConfig_Validate_RequiresAuth(t *testing.T) {
	cfg := Config{
		TenantURL: "https://tenant.console.ves.volterra.io",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth")
}

func TestConfig_Validate_RejectsBothAuth(t *testing.T) {
	cfg := Config{
		TenantURL:   "https://tenant.console.ves.volterra.io",
		APIToken:    "test-token",
		CertP12Path: "/path/to/cert.p12",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestConfig_Validate_AcceptsAPIToken(t *testing.T) {
	cfg := Config{
		TenantURL: "https://tenant.console.ves.volterra.io",
		APIToken:  "test-token",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_AcceptsP12(t *testing.T) {
	cfg := Config{
		TenantURL:    "https://tenant.console.ves.volterra.io",
		CertP12Path:  "/path/to/cert.p12",
		CertPassword: "pass",
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_StripsTrailingSlash(t *testing.T) {
	cfg := Config{
		TenantURL: "https://tenant.console.ves.volterra.io/",
		APIToken:  "test-token",
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "https://tenant.console.ves.volterra.io", cfg.TenantURL)
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		TenantURL: "https://tenant.console.ves.volterra.io",
		APIToken:  "test-token",
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.HTTPTimeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, float64(2), cfg.RateLimits.DefaultRPS)
	assert.Equal(t, 5, cfg.RateLimits.DefaultBurst)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestConfig
```

Expected: FAIL — `Config` type not defined.

- [ ] **Step 3: Implement config**

Create `internal/xcclient/config.go`:

```go
package xcclient

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	TenantURL    string
	APIToken     string
	CertP12Path  string
	CertPassword string
	RateLimits   RateLimitConfig
	HTTPTimeout  time.Duration
	MaxRetries   int
}

type RateLimitConfig struct {
	DefaultRPS   float64
	DefaultBurst int
	Overrides    map[string]EndpointLimit
}

type EndpointLimit struct {
	RPS   float64
	Burst int
}

func (c *Config) Validate() error {
	if c.TenantURL == "" {
		return fmt.Errorf("TenantURL is required")
	}
	c.TenantURL = strings.TrimRight(c.TenantURL, "/")

	hasToken := c.APIToken != ""
	hasP12 := c.CertP12Path != ""

	if hasToken && hasP12 {
		return fmt.Errorf("APIToken and CertP12Path are mutually exclusive")
	}
	if !hasToken && !hasP12 {
		return fmt.Errorf("one auth method required: set APIToken or CertP12Path")
	}

	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = 30 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 5
	}
	if c.RateLimits.DefaultRPS == 0 {
		c.RateLimits.DefaultRPS = 2
	}
	if c.RateLimits.DefaultBurst == 0 {
		c.RateLimits.DefaultBurst = 5
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestConfig
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/config.go internal/xcclient/config_test.go
git commit -m "Add Config struct with validation, defaults, and dual auth enforcement"
```

---

### Task 4: Shared Types (ObjectMeta, SystemMeta)

**Files:**
- Create: `internal/xcclient/types.go`
- Create: `internal/xcclient/types_test.go`

- [ ] **Step 1: Write failing tests for JSON round-tripping**

Create `internal/xcclient/types_test.go`:

```go
package xcclient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectMeta_JSON_RoundTrip(t *testing.T) {
	meta := ObjectMeta{
		Name:      "test-pool",
		Namespace: "default",
		Labels:    map[string]string{"app": "web"},
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded ObjectMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-pool", decoded.Name)
	assert.Equal(t, "default", decoded.Namespace)
	assert.Equal(t, "web", decoded.Labels["app"])
}

func TestObjectMeta_ResourceVersion_Included(t *testing.T) {
	meta := ObjectMeta{
		Name:            "test-pool",
		Namespace:       "default",
		ResourceVersion: "12345",
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"resource_version"`)
	assert.Contains(t, string(data), `"12345"`)
}

func TestObjectMeta_ResourceVersion_OmittedWhenEmpty(t *testing.T) {
	meta := ObjectMeta{
		Name:      "test-pool",
		Namespace: "default",
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "resource_version")
}

func TestSystemMeta_Deserialize(t *testing.T) {
	raw := `{
		"uid": "abc-123",
		"creation_timestamp": "2026-04-20T12:34:56Z",
		"modification_timestamp": "2026-04-20T12:35:00Z",
		"creator_id": "admin@example.com",
		"tenant": "my-tenant"
	}`

	var sm SystemMeta
	err := json.Unmarshal([]byte(raw), &sm)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", sm.UID)
	assert.Equal(t, "admin@example.com", sm.CreatorID)
	assert.Equal(t, "my-tenant", sm.Tenant)
}

func TestObjectRef_JSON(t *testing.T) {
	ref := ObjectRef{
		Name:      "my-pool",
		Namespace: "production",
		Tenant:    "my-tenant-abc",
	}

	data, err := json.Marshal(ref)
	require.NoError(t, err)

	var decoded ObjectRef
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, ref, decoded)
}

func TestResourcePath_Constants(t *testing.T) {
	assert.Equal(t, "origin_pools", ResourceOriginPool)
	assert.Equal(t, "http_loadbalancers", ResourceHTTPLoadBalancer)
	assert.Equal(t, "tcp_loadbalancers", ResourceTCPLoadBalancer)
	assert.Equal(t, "app_firewalls", ResourceAppFirewall)
	assert.Equal(t, "healthchecks", ResourceHealthCheck)
	assert.Equal(t, "service_policys", ResourceServicePolicy)
	assert.Equal(t, "rate_limiters", ResourceRateLimiter)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run "TestObjectMeta|TestSystemMeta|TestObjectRef|TestResourcePath"
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement shared types**

Create `internal/xcclient/types.go`:

```go
package xcclient

import "encoding/json"

const (
	ResourceOriginPool       = "origin_pools"
	ResourceHTTPLoadBalancer = "http_loadbalancers"
	ResourceTCPLoadBalancer  = "tcp_loadbalancers"
	ResourceAppFirewall      = "app_firewalls"
	ResourceHealthCheck      = "healthchecks"
	ResourceServicePolicy    = "service_policys"
	ResourceRateLimiter      = "rate_limiters"
)

type ObjectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	Description     string            `json:"description,omitempty"`
	Disable         bool              `json:"disable,omitempty"`
	ResourceVersion string            `json:"resource_version,omitempty"`
	UID             string            `json:"uid,omitempty"`
}

type SystemMeta struct {
	UID                   string `json:"uid,omitempty"`
	CreationTimestamp     string `json:"creation_timestamp,omitempty"`
	ModificationTimestamp string `json:"modification_timestamp,omitempty"`
	CreatorID             string `json:"creator_id,omitempty"`
	CreatorClass          string `json:"creator_class,omitempty"`
	Tenant                string `json:"tenant,omitempty"`
}

type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

type ObjectEnvelope struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           json.RawMessage `json:"spec"`
}

type ListResponse struct {
	Items []json.RawMessage `json:"items"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run "TestObjectMeta|TestSystemMeta|TestObjectRef|TestResourcePath"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/types.go internal/xcclient/types_test.go
git commit -m "Add shared types: ObjectMeta, SystemMeta, ObjectRef, resource path constants"
```

---

### Task 5: Per-Endpoint Rate Limiter

**Files:**
- Create: `internal/xcclient/ratelimit.go`
- Create: `internal/xcclient/ratelimit_test.go`

- [ ] **Step 1: Write failing tests for rate limiter**

Create `internal/xcclient/ratelimit_test.go`:

```go
package xcclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointRateLimiter_IsolatesEndpoints(t *testing.T) {
	cfg := RateLimitConfig{
		DefaultRPS:   100,
		DefaultBurst: 1,
	}
	rl := NewEndpointRateLimiter(cfg)

	ctx := context.Background()

	err := rl.Wait(ctx, "origin_pools")
	require.NoError(t, err)

	err = rl.Wait(ctx, "http_loadbalancers")
	require.NoError(t, err)
}

func TestEndpointRateLimiter_UsesDefaults(t *testing.T) {
	cfg := RateLimitConfig{
		DefaultRPS:   1,
		DefaultBurst: 1,
	}
	rl := NewEndpointRateLimiter(cfg)

	ctx := context.Background()

	start := time.Now()
	err := rl.Wait(ctx, "origin_pools")
	require.NoError(t, err)

	err = rl.Wait(ctx, "origin_pools")
	require.NoError(t, err)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

func TestEndpointRateLimiter_OverridePerEndpoint(t *testing.T) {
	cfg := RateLimitConfig{
		DefaultRPS:   1,
		DefaultBurst: 1,
		Overrides: map[string]EndpointLimit{
			"origin_pools": {RPS: 100, Burst: 10},
		},
	}
	rl := NewEndpointRateLimiter(cfg)

	ctx := context.Background()

	start := time.Now()
	for i := 0; i < 5; i++ {
		err := rl.Wait(ctx, "origin_pools")
		require.NoError(t, err)
	}
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestEndpointRateLimiter_RespectsContextCancellation(t *testing.T) {
	cfg := RateLimitConfig{
		DefaultRPS:   0.1,
		DefaultBurst: 1,
	}
	rl := NewEndpointRateLimiter(cfg)

	ctx := context.Background()
	err := rl.Wait(ctx, "origin_pools")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = rl.Wait(ctx, "origin_pools")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestEndpointRateLimiter
```

Expected: FAIL — `NewEndpointRateLimiter` not defined.

- [ ] **Step 3: Implement rate limiter**

Create `internal/xcclient/ratelimit.go`:

```go
package xcclient

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type EndpointRateLimiter struct {
	config   RateLimitConfig
	limiters sync.Map
}

func NewEndpointRateLimiter(cfg RateLimitConfig) *EndpointRateLimiter {
	return &EndpointRateLimiter{config: cfg}
}

func (e *EndpointRateLimiter) Wait(ctx context.Context, endpoint string) error {
	limiter := e.getLimiter(endpoint)
	return limiter.Wait(ctx)
}

func (e *EndpointRateLimiter) getLimiter(endpoint string) *rate.Limiter {
	if v, ok := e.limiters.Load(endpoint); ok {
		return v.(*rate.Limiter)
	}

	rps := e.config.DefaultRPS
	burst := e.config.DefaultBurst
	if override, ok := e.config.Overrides[endpoint]; ok {
		rps = override.RPS
		burst = override.Burst
	}

	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	actual, _ := e.limiters.LoadOrStore(endpoint, limiter)
	return actual.(*rate.Limiter)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestEndpointRateLimiter
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/ratelimit.go internal/xcclient/ratelimit_test.go
git commit -m "Add per-endpoint token bucket rate limiter with configurable overrides"
```

---

### Task 6: Prometheus Metrics

**Files:**
- Create: `internal/xcclient/metrics.go`

- [ ] **Step 1: Implement metrics**

Create `internal/xcclient/metrics.go`:

```go
package xcclient

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	RequestsTotal     *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	RateLimitHits     *prometheus.CounterVec
	RetriesTotal      *prometheus.CounterVec
	UpdatesSkipped    *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "f5xc_api_requests_total",
			Help: "Total number of F5 XC API requests",
		}, []string{"endpoint", "method", "status_code"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "f5xc_api_request_duration_seconds",
			Help:    "Duration of F5 XC API requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint", "method"}),

		RateLimitHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "f5xc_api_rate_limit_hits_total",
			Help: "Number of 429 rate limit responses received",
		}, []string{"endpoint"}),

		RetriesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "f5xc_api_retries_total",
			Help: "Total number of retried requests",
		}, []string{"endpoint", "reason"}),

		UpdatesSkipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "f5xc_api_updates_skipped_total",
			Help: "PUT requests skipped because NeedsUpdate returned false",
		}, []string{"endpoint"}),
	}

	if reg != nil {
		reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.RateLimitHits, m.RetriesTotal, m.UpdatesSkipped)
	}

	return m
}
```

No separate test file — metrics registration is validated by the client tests in Task 7.

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go build ./internal/xcclient/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/xcclient/metrics.go
git commit -m "Add Prometheus metric definitions for XC API client"
```

---

### Task 7: Client Core (Constructor, Auth, `do()` Helper, Retry)

**Files:**
- Create: `internal/xcclient/client.go`
- Create: `internal/xcclient/client_test.go`

- [ ] **Step 1: Write failing tests for client construction and auth**

Create `internal/xcclient/client_test.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_APIToken(t *testing.T) {
	cfg := Config{
		TenantURL: "https://tenant.console.ves.volterra.io",
		APIToken:  "test-token-123",
	}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_InvalidConfig(t *testing.T) {
	cfg := Config{}
	_, err := NewClient(cfg, logr.Discard(), nil)
	assert.Error(t, err)
}

func TestClient_APIToken_Header(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]string{"name": "test"},
			"spec":     map[string]interface{}{},
		})
	}))
	defer srv.Close()

	cfg := Config{
		TenantURL: srv.URL,
		APIToken:  "my-secret-token",
	}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	var result json.RawMessage
	err = client.do(context.Background(), http.MethodGet, "origin_pools", "default", "test", nil, &result)
	require.NoError(t, err)
	assert.Equal(t, "APIToken my-secret-token", gotHeader)
}

func TestClient_BuildsCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer srv.Close()

	cfg := Config{TenantURL: srv.URL, APIToken: "tok"}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	client.do(context.Background(), http.MethodGet, "origin_pools", "prod", "my-pool", nil, nil)
	assert.Equal(t, "/api/config/namespaces/prod/origin_pools/my-pool", gotPath)
}

func TestClient_BuildsListURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer srv.Close()

	cfg := Config{TenantURL: srv.URL, APIToken: "tok"}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	client.do(context.Background(), http.MethodGet, "service_policys", "default", "", nil, nil)
	assert.Equal(t, "/api/config/namespaces/default/service_policys", gotPath)
}

func TestClient_MapsErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"404", 404, ErrNotFound},
		{"401", 401, ErrAuth},
		{"409", 409, ErrConflict},
		{"429", 429, ErrRateLimited},
		{"500", 500, ErrServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "error", tt.statusCode)
			}))
			defer srv.Close()

			cfg := Config{
				TenantURL:  srv.URL,
				APIToken:   "tok",
				MaxRetries: 1,
			}
			client, err := NewClient(cfg, logr.Discard(), nil)
			require.NoError(t, err)

			err = client.do(context.Background(), http.MethodGet, "origin_pools", "default", "test", nil, nil)
			assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
		})
	}
}

func TestClient_Retry429_ExponentialBackoff(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count <= 2 {
			http.Error(w, "rate limited", 429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]string{"name": "test"},
			"spec":     map[string]interface{}{},
		})
	}))
	defer srv.Close()

	cfg := Config{
		TenantURL:  srv.URL,
		APIToken:   "tok",
		MaxRetries: 5,
	}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	client.baseDelay = 10 * time.Millisecond

	var result json.RawMessage
	err = client.do(context.Background(), http.MethodGet, "origin_pools", "default", "test", nil, &result)
	require.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestClient_Retry429_ExhaustsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", 429)
	}))
	defer srv.Close()

	cfg := Config{
		TenantURL:  srv.URL,
		APIToken:   "tok",
		MaxRetries: 3,
	}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	client.baseDelay = 1 * time.Millisecond

	err = client.do(context.Background(), http.MethodGet, "origin_pools", "default", "test", nil, nil)
	assert.True(t, errors.Is(err, ErrRateLimited))
}

func TestClient_NoRetryOnNon429(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "not found", 404)
	}))
	defer srv.Close()

	cfg := Config{TenantURL: srv.URL, APIToken: "tok", MaxRetries: 5}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	client.do(context.Background(), http.MethodGet, "origin_pools", "default", "test", nil, nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestClient_SendsJSONBody(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"metadata":{"name":"test"},"spec":{}}`)
	}))
	defer srv.Close()

	cfg := Config{TenantURL: srv.URL, APIToken: "tok"}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)

	body := map[string]interface{}{"metadata": map[string]string{"name": "test"}, "spec": map[string]interface{}{"port": 8080}}
	var result json.RawMessage
	err = client.do(context.Background(), http.MethodPost, "origin_pools", "default", "", body, &result)
	require.NoError(t, err)

	assert.Contains(t, string(gotBody), `"port":8080`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run "TestNewClient|TestClient_" -count=1
```

Expected: FAIL — `NewClient` not defined.

- [ ] **Step 3: Implement client core**

Create `internal/xcclient/client.go`:

```go
package xcclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	gopkcs12 "software.sslmate.com/src/go-pkcs12"
)

type Client struct {
	httpClient  *http.Client
	tenantURL   string
	apiToken    string
	maxRetries  int
	baseDelay   time.Duration
	rateLimiter *EndpointRateLimiter
	metrics     *Metrics
	log         logr.Logger
}

func NewClient(cfg Config, log logr.Logger, reg prometheus.Registerer) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	if cfg.CertP12Path != "" {
		cert, err := loadP12(cfg.CertP12Path, cfg.CertPassword)
		if err != nil {
			return nil, fmt.Errorf("loading P12 certificate: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.HTTPTimeout,
		},
		tenantURL:   cfg.TenantURL,
		apiToken:    cfg.APIToken,
		maxRetries:  cfg.MaxRetries,
		baseDelay:   1 * time.Second,
		rateLimiter: NewEndpointRateLimiter(cfg.RateLimits),
		metrics:     NewMetrics(reg),
		log:         log.WithName("xcclient"),
	}, nil
}

func (c *Client) do(ctx context.Context, method, resource, namespace, name string, body, result interface{}) error {
	path := fmt.Sprintf("/api/config/namespaces/%s/%s", namespace, resource)
	if name != "" {
		path = fmt.Sprintf("%s/%s", path, name)
	}

	if err := c.rateLimiter.Wait(ctx, resource); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.backoffDelay(attempt)
			c.log.V(1).Info("retrying after backoff", "attempt", attempt, "delay", delay, "resource", resource)
			c.metrics.RetriesTotal.WithLabelValues(resource, "rate_limit").Inc()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.doOnce(ctx, method, path, resource, body, result)
		if err == nil {
			return nil
		}

		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
			c.metrics.RateLimitHits.WithLabelValues(resource).Inc()
			c.log.Info("rate limited by XC API", "endpoint", resource, "attempt", attempt+1)
			lastErr = err
			continue
		}

		return err
	}

	return lastErr
}

func (c *Client) doOnce(ctx context.Context, method, path, resource string, body, result interface{}) error {
	url := c.tenantURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "APIToken "+c.apiToken)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	statusStr := strconv.Itoa(resp.StatusCode)
	c.metrics.RequestsTotal.WithLabelValues(resource, method, statusStr).Inc()
	c.metrics.RequestDuration.WithLabelValues(resource, method).Observe(duration.Seconds())
	c.log.V(1).Info("XC API request", "method", method, "path", path, "status", resp.StatusCode, "duration", duration)

	if resp.StatusCode >= 300 {
		return StatusToError(resp.StatusCode, resource, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	base := c.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	jitter := time.Duration(rand.Int63n(int64(500 * time.Millisecond)))
	return base + jitter
}

func loadP12(path, password string) (tls.Certificate, error) {
	data, err := readFile(path)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("reading P12 file: %w", err)
	}

	privateKey, cert, _, err := gopkcs12.DecodeChain(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decoding P12: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}, nil
}

var readFile = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}
```

Wait — we need the `os` and `errors` imports. Let me fix the imports:

```go
package xcclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	gopkcs12 "software.sslmate.com/src/go-pkcs12"
)
```

The full file is the concatenation of the correct import block and the rest of the code from `type Client struct` onward.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run "TestNewClient|TestClient_" -count=1
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/client.go internal/xcclient/client_test.go
git commit -m "Add Client core: constructor, API token auth, do() helper with 429 retry and backoff"
```

---

### Task 8: Change Detection (NeedsUpdate)

**Files:**
- Create: `internal/xcclient/diff.go`
- Create: `internal/xcclient/diff_test.go`

- [ ] **Step 1: Write failing tests for change detection**

Create `internal/xcclient/diff_test.go`:

```go
package xcclient

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsUpdate_IdenticalObjects_ReturnsFalse(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1", "uid": "abc-123", "resource_version": "10"},
		"system_metadata": {"creation_timestamp": "2026-01-01T00:00:00Z", "uid": "abc-123"},
		"spec": {"port": 8080, "loadbalancer_algorithm": "ROUND_ROBIN"}
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080, "loadbalancer_algorithm": "ROUND_ROBIN"}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestNeedsUpdate_DifferentSpec_ReturnsTrue(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1", "uid": "abc-123"},
		"system_metadata": {"creation_timestamp": "2026-01-01T00:00:00Z"},
		"spec": {"port": 8080, "loadbalancer_algorithm": "ROUND_ROBIN"}
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 9090, "loadbalancer_algorithm": "ROUND_ROBIN"}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestNeedsUpdate_IgnoresSystemMetadata(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1", "uid": "abc-123", "resource_version": "99"},
		"system_metadata": {"creation_timestamp": "2026-01-01T00:00:00Z", "modification_timestamp": "2026-01-02T00:00:00Z", "creator_id": "admin@test.com", "tenant": "t1"},
		"spec": {"port": 8080}
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestNeedsUpdate_IgnoresMetadataUID(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1", "uid": "server-assigned-uid", "resource_version": "5"},
		"spec": {"port": 8080}
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestNeedsUpdate_DetectsLabelChange(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1", "labels": {"app": "old"}},
		"spec": {"port": 8080}
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1", "labels": {"app": "new"}},
		"spec": {"port": 8080}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestNeedsUpdate_IgnoresStatusField(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080},
		"status": [{"conditions": [{"type": "Ready"}]}]
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestNeedsUpdate_IgnoresReferringObjects(t *testing.T) {
	current := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080},
		"referring_objects": [{"kind": "http_loadbalancer", "name": "lb1"}]
	}`)
	desired := json.RawMessage(`{
		"metadata": {"name": "pool1"},
		"spec": {"port": 8080}
	}`)

	changed, err := NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, changed)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestNeedsUpdate
```

Expected: FAIL — `NeedsUpdate` not defined.

- [ ] **Step 3: Implement change detection**

Create `internal/xcclient/diff.go`:

```go
package xcclient

import (
	"encoding/json"
	"fmt"
	"reflect"
)

var serverManagedTopLevel = map[string]bool{
	"system_metadata":   true,
	"status":            true,
	"referring_objects":  true,
	"create_form":       true,
	"replace_form":      true,
}

var serverManagedMetaFields = map[string]bool{
	"uid":              true,
	"resource_version": true,
}

func NeedsUpdate(current, desired json.RawMessage) (bool, error) {
	var currentMap, desiredMap map[string]json.RawMessage
	if err := json.Unmarshal(current, &currentMap); err != nil {
		return false, fmt.Errorf("unmarshaling current: %w", err)
	}
	if err := json.Unmarshal(desired, &desiredMap); err != nil {
		return false, fmt.Errorf("unmarshaling desired: %w", err)
	}

	normalizedCurrent, err := stripServerManaged(currentMap)
	if err != nil {
		return false, err
	}
	normalizedDesired, err := stripServerManaged(desiredMap)
	if err != nil {
		return false, err
	}

	currentBytes, err := json.Marshal(normalizedCurrent)
	if err != nil {
		return false, fmt.Errorf("remarshaling current: %w", err)
	}
	desiredBytes, err := json.Marshal(normalizedDesired)
	if err != nil {
		return false, fmt.Errorf("remarshaling desired: %w", err)
	}

	var currentNorm, desiredNorm interface{}
	json.Unmarshal(currentBytes, &currentNorm)
	json.Unmarshal(desiredBytes, &desiredNorm)

	return !reflect.DeepEqual(currentNorm, desiredNorm), nil
}

func stripServerManaged(obj map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	result := make(map[string]json.RawMessage)

	for key, val := range obj {
		if serverManagedTopLevel[key] {
			continue
		}
		if key == "metadata" {
			stripped, err := stripMetaFields(val)
			if err != nil {
				return nil, err
			}
			result[key] = stripped
			continue
		}
		result[key] = val
	}

	return result, nil
}

func stripMetaFields(raw json.RawMessage) (json.RawMessage, error) {
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	result := make(map[string]json.RawMessage)
	for key, val := range meta {
		if serverManagedMetaFields[key] {
			continue
		}
		result[key] = val
	}

	return json.Marshal(result)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestNeedsUpdate
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/diff.go internal/xcclient/diff_test.go
git commit -m "Add NeedsUpdate change detection with server-managed field stripping"
```

---

### Task 9: Fake XC API Server (Test Utility)

**Files:**
- Create: `internal/xcclient/testutil/fakeserver.go`

- [ ] **Step 1: Implement the fake server**

Create `internal/xcclient/testutil/fakeserver.go`:

```go
package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

type StoredObject struct {
	Metadata       map[string]interface{} `json:"metadata"`
	SystemMetadata map[string]interface{} `json:"system_metadata,omitempty"`
	Spec           map[string]interface{} `json:"spec"`
}

type FakeXCServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	objects  map[string]StoredObject // keyed by "resource/namespace/name"
	requests []RecordedRequest
	errors   map[string]ErrorSpec // keyed by "METHOD resource/namespace/name" — inject errors
}

type RecordedRequest struct {
	Method   string
	Path     string
	Body     json.RawMessage
}

type ErrorSpec struct {
	StatusCode int
	Body       string
	Times      int // 0 = forever, >0 = count down
}

func NewFakeXCServer() *FakeXCServer {
	f := &FakeXCServer{
		objects: make(map[string]StoredObject),
		errors:  make(map[string]ErrorSpec),
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handler))
	return f
}

func (f *FakeXCServer) Close() {
	f.Server.Close()
}

func (f *FakeXCServer) URL() string {
	return f.Server.URL
}

func (f *FakeXCServer) InjectError(method, resource, namespace, name string, spec ErrorSpec) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%s %s/%s/%s", method, resource, namespace, name)
	f.errors[key] = spec
}

func (f *FakeXCServer) ClearErrors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors = make(map[string]ErrorSpec)
}

func (f *FakeXCServer) Requests() []RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]RecordedRequest, len(f.requests))
	copy(cp, f.requests)
	return cp
}

func (f *FakeXCServer) ClearRequests() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = nil
}

func (f *FakeXCServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := readBody(r)

	f.mu.Lock()
	f.requests = append(f.requests, RecordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})

	resource, namespace, name := parsePath(r.URL.Path)
	errKey := fmt.Sprintf("%s %s/%s/%s", r.Method, resource, namespace, name)
	if spec, ok := f.errors[errKey]; ok {
		if spec.Times > 0 {
			spec.Times--
			if spec.Times == 0 {
				delete(f.errors, errKey)
			} else {
				f.errors[errKey] = spec
			}
		}
		f.mu.Unlock()
		http.Error(w, spec.Body, spec.StatusCode)
		return
	}
	f.mu.Unlock()

	objKey := fmt.Sprintf("%s/%s/%s", resource, namespace, name)

	switch r.Method {
	case http.MethodPost:
		f.handleCreate(w, resource, namespace, body)
	case http.MethodGet:
		if name == "" {
			f.handleList(w, resource, namespace)
		} else {
			f.handleGet(w, objKey)
		}
	case http.MethodPut:
		f.handleReplace(w, objKey, body)
	case http.MethodDelete:
		f.handleDelete(w, objKey)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *FakeXCServer) handleCreate(w http.ResponseWriter, resource, namespace string, body json.RawMessage) {
	var obj StoredObject
	json.Unmarshal(body, &obj)

	name, _ := obj.Metadata["name"].(string)
	key := fmt.Sprintf("%s/%s/%s", resource, namespace, name)

	f.mu.Lock()
	if _, exists := f.objects[key]; exists {
		f.mu.Unlock()
		http.Error(w, "already exists", http.StatusConflict)
		return
	}

	obj.SystemMetadata = map[string]interface{}{
		"uid":                    fmt.Sprintf("fake-uid-%s", name),
		"creation_timestamp":     "2026-04-20T12:00:00Z",
		"modification_timestamp": "2026-04-20T12:00:00Z",
		"tenant":                 "fake-tenant",
	}
	if obj.Metadata["resource_version"] == nil {
		obj.Metadata["resource_version"] = "1"
	}
	obj.Metadata["uid"] = obj.SystemMetadata["uid"]

	f.objects[key] = obj
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func (f *FakeXCServer) handleGet(w http.ResponseWriter, key string) {
	f.mu.Lock()
	obj, exists := f.objects[key]
	f.mu.Unlock()

	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func (f *FakeXCServer) handleList(w http.ResponseWriter, resource, namespace string) {
	f.mu.Lock()
	prefix := fmt.Sprintf("%s/%s/", resource, namespace)
	var items []StoredObject
	for key, obj := range f.objects {
		if strings.HasPrefix(key, prefix) {
			items = append(items, obj)
		}
	}
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
}

func (f *FakeXCServer) handleReplace(w http.ResponseWriter, key string, body json.RawMessage) {
	f.mu.Lock()
	_, exists := f.objects[key]
	if !exists {
		f.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var obj StoredObject
	json.Unmarshal(body, &obj)
	obj.SystemMetadata = map[string]interface{}{
		"uid":                    fmt.Sprintf("fake-uid-updated"),
		"creation_timestamp":     "2026-04-20T12:00:00Z",
		"modification_timestamp": "2026-04-20T13:00:00Z",
		"tenant":                 "fake-tenant",
	}

	f.objects[key] = obj
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func (f *FakeXCServer) handleDelete(w http.ResponseWriter, key string) {
	f.mu.Lock()
	_, exists := f.objects[key]
	if !exists {
		f.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	delete(f.objects, key)
	f.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{}`)
}

func readBody(r *http.Request) (json.RawMessage, error) {
	if r.Body == nil {
		return nil, nil
	}
	data, err := json.RawMessage(nil), error(nil)
	buf := new(strings.Builder)
	_, err = strings.NewReader("").WriteTo(buf)
	_ = buf

	raw := make([]byte, 0)
	raw, err = readAll(r.Body)
	return raw, err
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

func parsePath(path string) (resource, namespace, name string) {
	// Path: /api/config/namespaces/{ns}/{resource}[/{name}]
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// parts: [api, config, namespaces, {ns}, {resource}, {name}?]
	if len(parts) >= 5 {
		namespace = parts[3]
		resource = parts[4]
	}
	if len(parts) >= 6 {
		name = parts[5]
	}
	return
}
```

Actually, the `readBody` function is overly complex. Let me simplify:

Replace the `readBody` and `readAll` functions with:

```go
func readBody(r *http.Request) (json.RawMessage, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
```

And add `"io"` to the imports. Remove the `readAll` function entirely.

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go build ./internal/xcclient/testutil/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/xcclient/testutil/fakeserver.go
git commit -m "Add fake XC API server for httptest-based integration tests"
```

---

### Task 10: OriginPool Resource (Types + CRUD + Tests)

**Files:**
- Create: `internal/xcclient/originpool.go`
- Create: `internal/xcclient/originpool_test.go`

- [ ] **Step 1: Write failing tests for OriginPool CRUD**

Create `internal/xcclient/originpool_test.go`:

```go
package xcclient

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestOriginPool_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	client := newTestClient(t, fake.URL())

	create := &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "web-pool", Namespace: "default"},
		Spec: OriginPoolSpec{
			Port:                    8080,
			LoadBalancerAlgorithm:   "ROUND_ROBIN",
			OriginServers: []OriginServer{
				{PublicName: &PublicName{DNSName: "origin.example.com"}},
			},
		},
	}

	created, err := client.CreateOriginPool(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "web-pool", created.Metadata.Name)

	got, err := client.GetOriginPool(context.Background(), "default", "web-pool")
	require.NoError(t, err)
	assert.Equal(t, "web-pool", got.Metadata.Name)
}

func TestOriginPool_Replace(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	client := newTestClient(t, fake.URL())

	create := &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "web-pool", Namespace: "default"},
		Spec: OriginPoolSpec{
			Port: 8080,
			LoadBalancerAlgorithm: "ROUND_ROBIN",
		},
	}
	client.CreateOriginPool(context.Background(), "default", create)

	replace := &OriginPoolReplace{
		Metadata: ObjectMeta{Name: "web-pool", Namespace: "default"},
		Spec: OriginPoolSpec{
			Port: 9090,
			LoadBalancerAlgorithm: "LEAST_REQUEST",
		},
	}

	updated, err := client.ReplaceOriginPool(context.Background(), "default", "web-pool", replace)
	require.NoError(t, err)
	assert.Equal(t, "web-pool", updated.Metadata.Name)
}

func TestOriginPool_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	client := newTestClient(t, fake.URL())

	create := &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "web-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 8080},
	}
	client.CreateOriginPool(context.Background(), "default", create)

	err := client.DeleteOriginPool(context.Background(), "default", "web-pool")
	require.NoError(t, err)

	_, err = client.GetOriginPool(context.Background(), "default", "web-pool")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestOriginPool_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	client := newTestClient(t, fake.URL())

	for _, name := range []string{"pool-a", "pool-b"} {
		client.CreateOriginPool(context.Background(), "default", &OriginPoolCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     OriginPoolSpec{Port: 8080},
		})
	}

	pools, err := client.ListOriginPools(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, pools, 2)
}

func TestOriginPool_GetNotFound(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	client := newTestClient(t, fake.URL())

	_, err := client.GetOriginPool(context.Background(), "default", "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	cfg := Config{TenantURL: url, APIToken: "test-token"}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	return client
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestOriginPool
```

Expected: FAIL — OriginPool types not defined.

- [ ] **Step 3: Implement OriginPool types and CRUD methods**

Create `internal/xcclient/originpool.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type OriginPoolSpec struct {
	OriginServers         []OriginServer `json:"origin_servers,omitempty"`
	Port                  int            `json:"port,omitempty"`
	UseTLS                json.RawMessage `json:"use_tls,omitempty"`
	NoTLS                 json.RawMessage `json:"no_tls,omitempty"`
	LoadBalancerAlgorithm string         `json:"loadbalancer_algorithm,omitempty"`
	HealthCheck           []ObjectRef    `json:"healthcheck,omitempty"`
}

type OriginServer struct {
	PublicIP    *PublicIP    `json:"public_ip,omitempty"`
	PublicName  *PublicName  `json:"public_name,omitempty"`
	PrivateIP   *PrivateIP  `json:"private_ip,omitempty"`
	PrivateName *PrivateName `json:"private_name,omitempty"`
	K8SService  *K8SService `json:"k8s_service,omitempty"`
	ConsulService *ConsulService `json:"consul_service,omitempty"`
}

type PublicIP struct {
	IP string `json:"ip"`
}

type PublicName struct {
	DNSName string `json:"dns_name"`
}

type PrivateIP struct {
	IP   string    `json:"ip"`
	Site *ObjectRef `json:"site,omitempty"`
}

type PrivateName struct {
	DNSName string    `json:"dns_name"`
	Site    *ObjectRef `json:"site,omitempty"`
}

type K8SService struct {
	ServiceName   string    `json:"service_name"`
	ServiceNamespace string `json:"service_namespace,omitempty"`
	Site          *ObjectRef `json:"site,omitempty"`
	VK8SNetworks  json.RawMessage `json:"vk8s_networks,omitempty"`
	InsideNetwork json.RawMessage `json:"inside_network,omitempty"`
	OutsideNetwork json.RawMessage `json:"outside_network,omitempty"`
}

type ConsulService struct {
	ServiceName string    `json:"service_name"`
	Site        *ObjectRef `json:"site,omitempty"`
}

type OriginPoolCreate struct {
	Metadata ObjectMeta     `json:"metadata"`
	Spec     OriginPoolSpec `json:"spec"`
}

type OriginPoolReplace struct {
	Metadata ObjectMeta     `json:"metadata"`
	Spec     OriginPoolSpec `json:"spec"`
}

type OriginPool struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           OriginPoolSpec  `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

func (c *Client) CreateOriginPool(ctx context.Context, ns string, pool *OriginPoolCreate) (*OriginPool, error) {
	pool.Metadata.Namespace = ns
	var result OriginPool
	err := c.do(ctx, http.MethodPost, ResourceOriginPool, ns, "", pool, &result)
	return &result, err
}

func (c *Client) GetOriginPool(ctx context.Context, ns, name string) (*OriginPool, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceOriginPool, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}

	var result OriginPool
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceOriginPool(ctx context.Context, ns, name string, pool *OriginPoolReplace) (*OriginPool, error) {
	pool.Metadata.Namespace = ns
	pool.Metadata.Name = name
	var result OriginPool
	err := c.do(ctx, http.MethodPut, ResourceOriginPool, ns, name, pool, &result)
	return &result, err
}

func (c *Client) DeleteOriginPool(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceOriginPool, ns, name, nil, nil)
}

func (c *Client) ListOriginPools(ctx context.Context, ns string) ([]*OriginPool, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceOriginPool, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[OriginPool](raw)
}

func extractRawSpec(raw json.RawMessage) json.RawMessage {
	var envelope struct {
		Spec json.RawMessage `json:"spec"`
	}
	json.Unmarshal(raw, &envelope)
	return envelope.Spec
}

func unmarshalList[T any](raw json.RawMessage) ([]*T, error) {
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}

	results := make([]*T, 0, len(list.Items))
	for _, item := range list.Items {
		var obj T
		if err := json.Unmarshal(item, &obj); err != nil {
			return nil, err
		}
		results = append(results, &obj)
	}
	return results, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestOriginPool
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/originpool.go internal/xcclient/originpool_test.go
git commit -m "Add OriginPool types and CRUD methods with integration tests"
```

---

### Task 11: HealthCheck Resource

**Files:**
- Create: `internal/xcclient/healthcheck.go`
- Create: `internal/xcclient/healthcheck_test.go`

- [ ] **Step 1: Write failing tests for HealthCheck CRUD**

Create `internal/xcclient/healthcheck_test.go`:

```go
package xcclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestHealthCheck_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &HealthCheckCreate{
		Metadata: ObjectMeta{Name: "http-hc", Namespace: "shared"},
		Spec: HealthCheckSpec{
			HTTPHealthCheck: &HTTPHealthCheck{
				Path: "/health",
			},
			HealthyThreshold:   2,
			UnhealthyThreshold: 5,
			Interval:           15,
			Timeout:            3,
		},
	}

	created, err := client.CreateHealthCheck(context.Background(), "shared", create)
	require.NoError(t, err)
	assert.Equal(t, "http-hc", created.Metadata.Name)

	got, err := client.GetHealthCheck(context.Background(), "shared", "http-hc")
	require.NoError(t, err)
	assert.Equal(t, "http-hc", got.Metadata.Name)
}

func TestHealthCheck_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateHealthCheck(context.Background(), "shared", &HealthCheckCreate{
		Metadata: ObjectMeta{Name: "hc-del", Namespace: "shared"},
		Spec:     HealthCheckSpec{Interval: 10, Timeout: 3, HealthyThreshold: 1, UnhealthyThreshold: 3},
	})

	err := client.DeleteHealthCheck(context.Background(), "shared", "hc-del")
	require.NoError(t, err)

	_, err = client.GetHealthCheck(context.Background(), "shared", "hc-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestHealthCheck_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"hc-1", "hc-2", "hc-3"} {
		client.CreateHealthCheck(context.Background(), "shared", &HealthCheckCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "shared"},
			Spec:     HealthCheckSpec{Interval: 10, Timeout: 3, HealthyThreshold: 1, UnhealthyThreshold: 3},
		})
	}

	list, err := client.ListHealthChecks(context.Background(), "shared")
	require.NoError(t, err)
	assert.Len(t, list, 3)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestHealthCheck
```

Expected: FAIL — HealthCheck types not defined.

- [ ] **Step 3: Implement HealthCheck types and CRUD**

Create `internal/xcclient/healthcheck.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type HealthCheckSpec struct {
	HTTPHealthCheck  *HTTPHealthCheck  `json:"http_health_check,omitempty"`
	TCPHealthCheck   *TCPHealthCheck   `json:"tcp_health_check,omitempty"`
	HealthyThreshold   uint32 `json:"healthy_threshold,omitempty"`
	UnhealthyThreshold uint32 `json:"unhealthy_threshold,omitempty"`
	Interval           uint32 `json:"interval,omitempty"`
	Timeout            uint32 `json:"timeout,omitempty"`
	JitterPercent      uint32 `json:"jitter_percent,omitempty"`
}

type HTTPHealthCheck struct {
	Path                string   `json:"path,omitempty"`
	UseHTTP2            bool     `json:"use_http2,omitempty"`
	ExpectedStatusCodes []string `json:"expected_status_codes,omitempty"`
}

type TCPHealthCheck struct {
	Send    string `json:"send,omitempty"`
	Receive string `json:"receive,omitempty"`
}

type HealthCheckCreate struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     HealthCheckSpec `json:"spec"`
}

type HealthCheckReplace struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     HealthCheckSpec `json:"spec"`
}

type HealthCheck struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           HealthCheckSpec `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

func (c *Client) CreateHealthCheck(ctx context.Context, ns string, hc *HealthCheckCreate) (*HealthCheck, error) {
	hc.Metadata.Namespace = ns
	var result HealthCheck
	err := c.do(ctx, http.MethodPost, ResourceHealthCheck, ns, "", hc, &result)
	return &result, err
}

func (c *Client) GetHealthCheck(ctx context.Context, ns, name string) (*HealthCheck, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceHealthCheck, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result HealthCheck
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceHealthCheck(ctx context.Context, ns, name string, hc *HealthCheckReplace) (*HealthCheck, error) {
	hc.Metadata.Namespace = ns
	hc.Metadata.Name = name
	var result HealthCheck
	err := c.do(ctx, http.MethodPut, ResourceHealthCheck, ns, name, hc, &result)
	return &result, err
}

func (c *Client) DeleteHealthCheck(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceHealthCheck, ns, name, nil, nil)
}

func (c *Client) ListHealthChecks(ctx context.Context, ns string) ([]*HealthCheck, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceHealthCheck, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[HealthCheck](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestHealthCheck
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/healthcheck.go internal/xcclient/healthcheck_test.go
git commit -m "Add HealthCheck types and CRUD methods"
```

---

### Task 12: AppFirewall Resource

**Files:**
- Create: `internal/xcclient/appfirewall.go`
- Create: `internal/xcclient/appfirewall_test.go`

- [ ] **Step 1: Write failing tests for AppFirewall CRUD**

Create `internal/xcclient/appfirewall_test.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestAppFirewall_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &AppFirewallCreate{
		Metadata: ObjectMeta{Name: "base-waf", Namespace: "shared"},
		Spec: AppFirewallSpec{
			DefaultDetectionSettings: json.RawMessage(`{}`),
			Monitoring:               json.RawMessage(`{}`),
			UseDefaultBlockingPage:   json.RawMessage(`{}`),
			AllowAllResponseCodes:    json.RawMessage(`{}`),
			DefaultAnonymization:     json.RawMessage(`{}`),
			DefaultBotSetting:        json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateAppFirewall(context.Background(), "shared", create)
	require.NoError(t, err)
	assert.Equal(t, "base-waf", created.Metadata.Name)

	got, err := client.GetAppFirewall(context.Background(), "shared", "base-waf")
	require.NoError(t, err)
	assert.Equal(t, "base-waf", got.Metadata.Name)
}

func TestAppFirewall_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateAppFirewall(context.Background(), "shared", &AppFirewallCreate{
		Metadata: ObjectMeta{Name: "waf-del", Namespace: "shared"},
		Spec:     AppFirewallSpec{Monitoring: json.RawMessage(`{}`)},
	})

	err := client.DeleteAppFirewall(context.Background(), "shared", "waf-del")
	require.NoError(t, err)

	_, err = client.GetAppFirewall(context.Background(), "shared", "waf-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAppFirewall_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"waf-1", "waf-2"} {
		client.CreateAppFirewall(context.Background(), "shared", &AppFirewallCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "shared"},
			Spec:     AppFirewallSpec{Monitoring: json.RawMessage(`{}`)},
		})
	}

	list, err := client.ListAppFirewalls(context.Background(), "shared")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestAppFirewall
```

Expected: FAIL — AppFirewall types not defined.

- [ ] **Step 3: Implement AppFirewall types and CRUD**

Create `internal/xcclient/appfirewall.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type AppFirewallSpec struct {
	// Detection — OneOf
	DefaultDetectionSettings json.RawMessage `json:"default_detection_settings,omitempty"`
	DetectionSettings        json.RawMessage `json:"detection_settings,omitempty"`

	// Enforcement mode — OneOf
	Monitoring json.RawMessage `json:"monitoring,omitempty"`
	Blocking   json.RawMessage `json:"blocking,omitempty"`

	// Blocking page — OneOf
	UseDefaultBlockingPage json.RawMessage `json:"use_default_blocking_page,omitempty"`
	BlockingPage           json.RawMessage `json:"blocking_page,omitempty"`

	// Response codes
	AllowAllResponseCodes json.RawMessage `json:"allow_all_response_codes,omitempty"`
	AllowedResponseCodes  json.RawMessage `json:"allowed_response_codes,omitempty"`

	// Bot setting
	DefaultBotSetting json.RawMessage `json:"default_bot_setting,omitempty"`
	BotProtectionSetting json.RawMessage `json:"bot_protection_setting,omitempty"`

	// Anonymization
	DefaultAnonymization json.RawMessage `json:"default_anonymization,omitempty"`
	CustomAnonymization  json.RawMessage `json:"custom_anonymization,omitempty"`

	// Loadbalancer setting
	UseLoadbalancerSetting json.RawMessage `json:"use_loadbalancer_setting,omitempty"`
}

type AppFirewallCreate struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     AppFirewallSpec `json:"spec"`
}

type AppFirewallReplace struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     AppFirewallSpec `json:"spec"`
}

type AppFirewall struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           AppFirewallSpec `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

func (c *Client) CreateAppFirewall(ctx context.Context, ns string, fw *AppFirewallCreate) (*AppFirewall, error) {
	fw.Metadata.Namespace = ns
	var result AppFirewall
	err := c.do(ctx, http.MethodPost, ResourceAppFirewall, ns, "", fw, &result)
	return &result, err
}

func (c *Client) GetAppFirewall(ctx context.Context, ns, name string) (*AppFirewall, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceAppFirewall, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result AppFirewall
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceAppFirewall(ctx context.Context, ns, name string, fw *AppFirewallReplace) (*AppFirewall, error) {
	fw.Metadata.Namespace = ns
	fw.Metadata.Name = name
	var result AppFirewall
	err := c.do(ctx, http.MethodPut, ResourceAppFirewall, ns, name, fw, &result)
	return &result, err
}

func (c *Client) DeleteAppFirewall(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceAppFirewall, ns, name, nil, nil)
}

func (c *Client) ListAppFirewalls(ctx context.Context, ns string) ([]*AppFirewall, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceAppFirewall, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[AppFirewall](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestAppFirewall
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/appfirewall.go internal/xcclient/appfirewall_test.go
git commit -m "Add AppFirewall types and CRUD methods with OneOf field support"
```

---

### Task 13: HTTPLoadBalancer Resource

**Files:**
- Create: `internal/xcclient/httplb.go`
- Create: `internal/xcclient/httplb_test.go`

- [ ] **Step 1: Write failing tests for HTTPLoadBalancer CRUD**

Create `internal/xcclient/httplb_test.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestHTTPLoadBalancer_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "web-lb", Namespace: "default"},
		Spec: HTTPLoadBalancerSpec{
			Domains: []string{"app.example.com"},
			HTTPSAutoCert: json.RawMessage(`{"http_redirect": true}`),
			DefaultRoutePools: []RoutePool{
				{
					Pool:   ObjectRef{Name: "web-pool", Namespace: "default"},
					Weight: 1,
				},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
			DisableBotDefense:          json.RawMessage(`{}`),
			DisableAPIDiscovery:        json.RawMessage(`{}`),
			DisableIPReputation:        json.RawMessage(`{}`),
			NoChallenge:                json.RawMessage(`{}`),
			RoundRobin:                 json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateHTTPLoadBalancer(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "web-lb", created.Metadata.Name)

	got, err := client.GetHTTPLoadBalancer(context.Background(), "default", "web-lb")
	require.NoError(t, err)
	assert.Equal(t, "web-lb", got.Metadata.Name)
}

func TestHTTPLoadBalancer_WithWAF(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "secure-lb", Namespace: "default"},
		Spec: HTTPLoadBalancerSpec{
			Domains:    []string{"secure.example.com"},
			HTTPSAutoCert: json.RawMessage(`{}`),
			AppFirewall: &ObjectRef{Name: "base-waf", Namespace: "shared"},
			DefaultRoutePools: []RoutePool{
				{Pool: ObjectRef{Name: "web-pool", Namespace: "default"}, Weight: 1},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateHTTPLoadBalancer(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "secure-lb", created.Metadata.Name)
}

func TestHTTPLoadBalancer_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateHTTPLoadBalancer(context.Background(), "default", &HTTPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "del-lb", Namespace: "default"},
		Spec:     HTTPLoadBalancerSpec{Domains: []string{"del.example.com"}},
	})

	err := client.DeleteHTTPLoadBalancer(context.Background(), "default", "del-lb")
	require.NoError(t, err)

	_, err = client.GetHTTPLoadBalancer(context.Background(), "default", "del-lb")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestHTTPLoadBalancer_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"lb-1", "lb-2"} {
		client.CreateHTTPLoadBalancer(context.Background(), "default", &HTTPLoadBalancerCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     HTTPLoadBalancerSpec{Domains: []string{name + ".example.com"}},
		})
	}

	list, err := client.ListHTTPLoadBalancers(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestHTTPLoadBalancer
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement HTTPLoadBalancer types and CRUD**

Create `internal/xcclient/httplb.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type HTTPLoadBalancerSpec struct {
	Domains           []string `json:"domains,omitempty"`
	DefaultRoutePools []RoutePool `json:"default_route_pools,omitempty"`
	Routes            json.RawMessage `json:"routes,omitempty"`

	// TLS — OneOf
	HTTP          json.RawMessage `json:"http,omitempty"`
	HTTPS         json.RawMessage `json:"https,omitempty"`
	HTTPSAutoCert json.RawMessage `json:"https_auto_cert,omitempty"`

	// WAF — OneOf
	DisableWAF  json.RawMessage `json:"disable_waf,omitempty"`
	AppFirewall *ObjectRef      `json:"app_firewall,omitempty"`

	// Bot defense — OneOf
	DisableBotDefense json.RawMessage `json:"disable_bot_defense,omitempty"`
	BotDefense        json.RawMessage `json:"bot_defense,omitempty"`

	// API discovery — OneOf
	DisableAPIDiscovery json.RawMessage `json:"disable_api_discovery,omitempty"`
	EnableAPIDiscovery  json.RawMessage `json:"enable_api_discovery,omitempty"`

	// IP reputation — OneOf
	DisableIPReputation json.RawMessage `json:"disable_ip_reputation,omitempty"`
	EnableIPReputation  json.RawMessage `json:"enable_ip_reputation,omitempty"`

	// Rate limit — OneOf
	DisableRateLimit json.RawMessage `json:"disable_rate_limit,omitempty"`
	RateLimit        json.RawMessage `json:"rate_limit,omitempty"`

	// Challenge — OneOf
	NoChallenge          json.RawMessage `json:"no_challenge,omitempty"`
	JSChallenge          json.RawMessage `json:"js_challenge,omitempty"`
	CaptchaChallenge     json.RawMessage `json:"captcha_challenge,omitempty"`
	PolicyBasedChallenge json.RawMessage `json:"policy_based_challenge,omitempty"`

	// LB algorithm — OneOf
	RoundRobin         json.RawMessage `json:"round_robin,omitempty"`
	LeastActive        json.RawMessage `json:"least_active,omitempty"`
	Random             json.RawMessage `json:"random,omitempty"`
	SourceIPStickiness json.RawMessage `json:"source_ip_stickiness,omitempty"`
	CookieStickiness   json.RawMessage `json:"cookie_stickiness,omitempty"`
	RingHash           json.RawMessage `json:"ring_hash,omitempty"`

	// Advertise — OneOf
	AdvertiseOnPublicDefaultVIP json.RawMessage `json:"advertise_on_public_default_vip,omitempty"`
	AdvertiseOnPublic           json.RawMessage `json:"advertise_on_public,omitempty"`
	AdvertiseCustom             json.RawMessage `json:"advertise_custom,omitempty"`
	DoNotAdvertise              json.RawMessage `json:"do_not_advertise,omitempty"`

	// Service policies
	ServicePoliciesFromNamespace json.RawMessage `json:"service_policies_from_namespace,omitempty"`
	ActiveServicePolicies        json.RawMessage `json:"active_service_policies,omitempty"`
	NoServicePolicies            json.RawMessage `json:"no_service_policies,omitempty"`

	// User identification
	UserIDClientIP json.RawMessage `json:"user_id_client_ip,omitempty"`
}

type RoutePool struct {
	Pool     ObjectRef `json:"pool"`
	Weight   uint32    `json:"weight,omitempty"`
	Priority uint32    `json:"priority,omitempty"`
}

type HTTPLoadBalancerCreate struct {
	Metadata ObjectMeta           `json:"metadata"`
	Spec     HTTPLoadBalancerSpec `json:"spec"`
}

type HTTPLoadBalancerReplace struct {
	Metadata ObjectMeta           `json:"metadata"`
	Spec     HTTPLoadBalancerSpec `json:"spec"`
}

type HTTPLoadBalancer struct {
	Metadata       ObjectMeta           `json:"metadata"`
	SystemMetadata SystemMeta           `json:"system_metadata,omitempty"`
	Spec           HTTPLoadBalancerSpec `json:"spec"`
	RawSpec        json.RawMessage      `json:"-"`
}

func (c *Client) CreateHTTPLoadBalancer(ctx context.Context, ns string, lb *HTTPLoadBalancerCreate) (*HTTPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	var result HTTPLoadBalancer
	err := c.do(ctx, http.MethodPost, ResourceHTTPLoadBalancer, ns, "", lb, &result)
	return &result, err
}

func (c *Client) GetHTTPLoadBalancer(ctx context.Context, ns, name string) (*HTTPLoadBalancer, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceHTTPLoadBalancer, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result HTTPLoadBalancer
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceHTTPLoadBalancer(ctx context.Context, ns, name string, lb *HTTPLoadBalancerReplace) (*HTTPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	lb.Metadata.Name = name
	var result HTTPLoadBalancer
	err := c.do(ctx, http.MethodPut, ResourceHTTPLoadBalancer, ns, name, lb, &result)
	return &result, err
}

func (c *Client) DeleteHTTPLoadBalancer(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceHTTPLoadBalancer, ns, name, nil, nil)
}

func (c *Client) ListHTTPLoadBalancers(ctx context.Context, ns string) ([]*HTTPLoadBalancer, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceHTTPLoadBalancer, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[HTTPLoadBalancer](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestHTTPLoadBalancer
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/httplb.go internal/xcclient/httplb_test.go
git commit -m "Add HTTPLoadBalancer types and CRUD with OneOf groups for all security/delivery options"
```

---

### Task 14: TCPLoadBalancer Resource

**Files:**
- Create: `internal/xcclient/tcplb.go`
- Create: `internal/xcclient/tcplb_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/xcclient/tcplb_test.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestTCPLoadBalancer_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &TCPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "tcp-lb", Namespace: "default"},
		Spec: TCPLoadBalancerSpec{
			Domains: []string{"tcp.example.com"},
			ListenPort: 5432,
			OriginPools: []RoutePool{
				{Pool: ObjectRef{Name: "db-pool", Namespace: "default"}, Weight: 1},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateTCPLoadBalancer(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "tcp-lb", created.Metadata.Name)

	got, err := client.GetTCPLoadBalancer(context.Background(), "default", "tcp-lb")
	require.NoError(t, err)
	assert.Equal(t, "tcp-lb", got.Metadata.Name)
}

func TestTCPLoadBalancer_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateTCPLoadBalancer(context.Background(), "default", &TCPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "tcp-del", Namespace: "default"},
		Spec:     TCPLoadBalancerSpec{ListenPort: 443},
	})

	err := client.DeleteTCPLoadBalancer(context.Background(), "default", "tcp-del")
	require.NoError(t, err)

	_, err = client.GetTCPLoadBalancer(context.Background(), "default", "tcp-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTCPLoadBalancer_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"tcp-1", "tcp-2"} {
		client.CreateTCPLoadBalancer(context.Background(), "default", &TCPLoadBalancerCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     TCPLoadBalancerSpec{ListenPort: 443},
		})
	}

	list, err := client.ListTCPLoadBalancers(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestTCPLoadBalancer
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement TCPLoadBalancer types and CRUD**

Create `internal/xcclient/tcplb.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type TCPLoadBalancerSpec struct {
	Domains     []string    `json:"domains,omitempty"`
	ListenPort  uint32      `json:"listen_port,omitempty"`
	OriginPools []RoutePool `json:"origin_pools,omitempty"`

	// TLS — OneOf
	NoTLS         json.RawMessage `json:"no_tls,omitempty"`
	TLSParameters json.RawMessage `json:"tls_parameters,omitempty"`
	TLSPassthrough json.RawMessage `json:"tls_tcp_passthrough,omitempty"`

	// Advertise — OneOf
	AdvertiseOnPublicDefaultVIP json.RawMessage `json:"advertise_on_public_default_vip,omitempty"`
	AdvertiseOnPublic           json.RawMessage `json:"advertise_on_public,omitempty"`
	AdvertiseCustom             json.RawMessage `json:"advertise_custom,omitempty"`
	DoNotAdvertise              json.RawMessage `json:"do_not_advertise,omitempty"`
}

type TCPLoadBalancerCreate struct {
	Metadata ObjectMeta          `json:"metadata"`
	Spec     TCPLoadBalancerSpec `json:"spec"`
}

type TCPLoadBalancerReplace struct {
	Metadata ObjectMeta          `json:"metadata"`
	Spec     TCPLoadBalancerSpec `json:"spec"`
}

type TCPLoadBalancer struct {
	Metadata       ObjectMeta          `json:"metadata"`
	SystemMetadata SystemMeta          `json:"system_metadata,omitempty"`
	Spec           TCPLoadBalancerSpec `json:"spec"`
	RawSpec        json.RawMessage     `json:"-"`
}

func (c *Client) CreateTCPLoadBalancer(ctx context.Context, ns string, lb *TCPLoadBalancerCreate) (*TCPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	var result TCPLoadBalancer
	err := c.do(ctx, http.MethodPost, ResourceTCPLoadBalancer, ns, "", lb, &result)
	return &result, err
}

func (c *Client) GetTCPLoadBalancer(ctx context.Context, ns, name string) (*TCPLoadBalancer, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceTCPLoadBalancer, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result TCPLoadBalancer
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceTCPLoadBalancer(ctx context.Context, ns, name string, lb *TCPLoadBalancerReplace) (*TCPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	lb.Metadata.Name = name
	var result TCPLoadBalancer
	err := c.do(ctx, http.MethodPut, ResourceTCPLoadBalancer, ns, name, lb, &result)
	return &result, err
}

func (c *Client) DeleteTCPLoadBalancer(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceTCPLoadBalancer, ns, name, nil, nil)
}

func (c *Client) ListTCPLoadBalancers(ctx context.Context, ns string) ([]*TCPLoadBalancer, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceTCPLoadBalancer, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[TCPLoadBalancer](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestTCPLoadBalancer
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/tcplb.go internal/xcclient/tcplb_test.go
git commit -m "Add TCPLoadBalancer types and CRUD methods"
```

---

### Task 15: ServicePolicy Resource

**Files:**
- Create: `internal/xcclient/servicepolicy.go`
- Create: `internal/xcclient/servicepolicy_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/xcclient/servicepolicy_test.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestServicePolicy_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: "deny-bad-ips", Namespace: "default"},
		Spec: ServicePolicySpec{
			Rules: []json.RawMessage{
				json.RawMessage(`{"metadata":{"name":"block-rule"},"spec":{"action":"DENY","ip_prefix_set":{"name":"bad-ips","namespace":"shared"}}}`),
			},
		},
	}

	created, err := client.CreateServicePolicy(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "deny-bad-ips", created.Metadata.Name)

	got, err := client.GetServicePolicy(context.Background(), "default", "deny-bad-ips")
	require.NoError(t, err)
	assert.Equal(t, "deny-bad-ips", got.Metadata.Name)
}

func TestServicePolicy_UsesIrregularPlural(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateServicePolicy(context.Background(), "default", &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: "test-sp", Namespace: "default"},
		Spec:     ServicePolicySpec{},
	})

	reqs := fake.Requests()
	require.NotEmpty(t, reqs)
	assert.Contains(t, reqs[0].Path, "service_policys")
}

func TestServicePolicy_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateServicePolicy(context.Background(), "default", &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: "sp-del", Namespace: "default"},
		Spec:     ServicePolicySpec{},
	})

	err := client.DeleteServicePolicy(context.Background(), "default", "sp-del")
	require.NoError(t, err)

	_, err = client.GetServicePolicy(context.Background(), "default", "sp-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestServicePolicy_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"sp-1", "sp-2"} {
		client.CreateServicePolicy(context.Background(), "default", &ServicePolicyCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     ServicePolicySpec{},
		})
	}

	list, err := client.ListServicePolicies(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestServicePolicy
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement ServicePolicy types and CRUD**

Create `internal/xcclient/servicepolicy.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type ServicePolicySpec struct {
	Rules []json.RawMessage `json:"rules,omitempty"`
	Algo  string            `json:"algo,omitempty"`
}

type ServicePolicyCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

type ServicePolicyReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

type ServicePolicy struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           ServicePolicySpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

func (c *Client) CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error) {
	sp.Metadata.Namespace = ns
	var result ServicePolicy
	err := c.do(ctx, http.MethodPost, ResourceServicePolicy, ns, "", sp, &result)
	return &result, err
}

func (c *Client) GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceServicePolicy, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result ServicePolicy
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceServicePolicy(ctx context.Context, ns, name string, sp *ServicePolicyReplace) (*ServicePolicy, error) {
	sp.Metadata.Namespace = ns
	sp.Metadata.Name = name
	var result ServicePolicy
	err := c.do(ctx, http.MethodPut, ResourceServicePolicy, ns, name, sp, &result)
	return &result, err
}

func (c *Client) DeleteServicePolicy(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceServicePolicy, ns, name, nil, nil)
}

func (c *Client) ListServicePolicies(ctx context.Context, ns string) ([]*ServicePolicy, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceServicePolicy, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[ServicePolicy](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestServicePolicy
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/servicepolicy.go internal/xcclient/servicepolicy_test.go
git commit -m "Add ServicePolicy types and CRUD (uses irregular plural service_policys)"
```

---

### Task 16: RateLimiter (XC Resource)

**Files:**
- Create: `internal/xcclient/xcratelimiter.go`
- Create: `internal/xcclient/xcratelimiter_test.go`

Note: named `xcratelimiter.go` to avoid confusion with our internal `ratelimit.go`.

- [ ] **Step 1: Write failing tests**

Create `internal/xcclient/xcratelimiter_test.go`:

```go
package xcclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestXCRateLimiter_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &XCRateLimiterCreate{
		Metadata: ObjectMeta{Name: "api-limiter", Namespace: "default"},
		Spec: XCRateLimiterSpec{
			Threshold: 1000,
			Unit:      "MINUTE",
			BurstMultiplier: 2,
		},
	}

	created, err := client.CreateRateLimiter(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "api-limiter", created.Metadata.Name)

	got, err := client.GetRateLimiter(context.Background(), "default", "api-limiter")
	require.NoError(t, err)
	assert.Equal(t, "api-limiter", got.Metadata.Name)
}

func TestXCRateLimiter_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateRateLimiter(context.Background(), "default", &XCRateLimiterCreate{
		Metadata: ObjectMeta{Name: "rl-del", Namespace: "default"},
		Spec:     XCRateLimiterSpec{Threshold: 100, Unit: "SECOND"},
	})

	err := client.DeleteRateLimiter(context.Background(), "default", "rl-del")
	require.NoError(t, err)

	_, err = client.GetRateLimiter(context.Background(), "default", "rl-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestXCRateLimiter_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"rl-1", "rl-2"} {
		client.CreateRateLimiter(context.Background(), "default", &XCRateLimiterCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     XCRateLimiterSpec{Threshold: 100, Unit: "SECOND"},
		})
	}

	list, err := client.ListRateLimiters(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestXCRateLimiter
```

Expected: FAIL — types not defined.

- [ ] **Step 3: Implement RateLimiter (XC resource) types and CRUD**

Create `internal/xcclient/xcratelimiter.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type XCRateLimiterSpec struct {
	Threshold       uint32 `json:"total_number,omitempty"`
	Unit            string `json:"unit,omitempty"`
	BurstMultiplier uint32 `json:"burst_multiplier,omitempty"`
}

type XCRateLimiterCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     XCRateLimiterSpec `json:"spec"`
}

type XCRateLimiterReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     XCRateLimiterSpec `json:"spec"`
}

type XCRateLimiter struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           XCRateLimiterSpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

func (c *Client) CreateRateLimiter(ctx context.Context, ns string, rl *XCRateLimiterCreate) (*XCRateLimiter, error) {
	rl.Metadata.Namespace = ns
	var result XCRateLimiter
	err := c.do(ctx, http.MethodPost, ResourceRateLimiter, ns, "", rl, &result)
	return &result, err
}

func (c *Client) GetRateLimiter(ctx context.Context, ns, name string) (*XCRateLimiter, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceRateLimiter, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result XCRateLimiter
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceRateLimiter(ctx context.Context, ns, name string, rl *XCRateLimiterReplace) (*XCRateLimiter, error) {
	rl.Metadata.Namespace = ns
	rl.Metadata.Name = name
	var result XCRateLimiter
	err := c.do(ctx, http.MethodPut, ResourceRateLimiter, ns, name, rl, &result)
	return &result, err
}

func (c *Client) DeleteRateLimiter(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceRateLimiter, ns, name, nil, nil)
}

func (c *Client) ListRateLimiters(ctx context.Context, ns string) ([]*XCRateLimiter, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceRateLimiter, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[XCRateLimiter](raw)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestXCRateLimiter
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/xcclient/xcratelimiter.go internal/xcclient/xcratelimiter_test.go
git commit -m "Add XC RateLimiter resource types and CRUD methods"
```

---

### Task 17: XCClient Interface + NeedsUpdate on Client

**Files:**
- Create: `internal/xcclient/interface.go`

- [ ] **Step 1: Write a compile-time interface check test**

Add to the bottom of `internal/xcclient/client_test.go`:

```go
func TestClient_ImplementsXCClient(t *testing.T) {
	var _ XCClient = (*Client)(nil)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestClient_ImplementsXCClient
```

Expected: FAIL — `XCClient` interface not defined, or `Client` doesn't implement it.

- [ ] **Step 3: Create interface definition and add NeedsUpdate to Client**

Create `internal/xcclient/interface.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
)

type XCClient interface {
	CreateOriginPool(ctx context.Context, ns string, pool *OriginPoolCreate) (*OriginPool, error)
	GetOriginPool(ctx context.Context, ns, name string) (*OriginPool, error)
	ReplaceOriginPool(ctx context.Context, ns, name string, pool *OriginPoolReplace) (*OriginPool, error)
	DeleteOriginPool(ctx context.Context, ns, name string) error
	ListOriginPools(ctx context.Context, ns string) ([]*OriginPool, error)

	CreateHTTPLoadBalancer(ctx context.Context, ns string, lb *HTTPLoadBalancerCreate) (*HTTPLoadBalancer, error)
	GetHTTPLoadBalancer(ctx context.Context, ns, name string) (*HTTPLoadBalancer, error)
	ReplaceHTTPLoadBalancer(ctx context.Context, ns, name string, lb *HTTPLoadBalancerReplace) (*HTTPLoadBalancer, error)
	DeleteHTTPLoadBalancer(ctx context.Context, ns, name string) error
	ListHTTPLoadBalancers(ctx context.Context, ns string) ([]*HTTPLoadBalancer, error)

	CreateTCPLoadBalancer(ctx context.Context, ns string, lb *TCPLoadBalancerCreate) (*TCPLoadBalancer, error)
	GetTCPLoadBalancer(ctx context.Context, ns, name string) (*TCPLoadBalancer, error)
	ReplaceTCPLoadBalancer(ctx context.Context, ns, name string, lb *TCPLoadBalancerReplace) (*TCPLoadBalancer, error)
	DeleteTCPLoadBalancer(ctx context.Context, ns, name string) error
	ListTCPLoadBalancers(ctx context.Context, ns string) ([]*TCPLoadBalancer, error)

	CreateAppFirewall(ctx context.Context, ns string, fw *AppFirewallCreate) (*AppFirewall, error)
	GetAppFirewall(ctx context.Context, ns, name string) (*AppFirewall, error)
	ReplaceAppFirewall(ctx context.Context, ns, name string, fw *AppFirewallReplace) (*AppFirewall, error)
	DeleteAppFirewall(ctx context.Context, ns, name string) error
	ListAppFirewalls(ctx context.Context, ns string) ([]*AppFirewall, error)

	CreateHealthCheck(ctx context.Context, ns string, hc *HealthCheckCreate) (*HealthCheck, error)
	GetHealthCheck(ctx context.Context, ns, name string) (*HealthCheck, error)
	ReplaceHealthCheck(ctx context.Context, ns, name string, hc *HealthCheckReplace) (*HealthCheck, error)
	DeleteHealthCheck(ctx context.Context, ns, name string) error
	ListHealthChecks(ctx context.Context, ns string) ([]*HealthCheck, error)

	CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error)
	GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error)
	ReplaceServicePolicy(ctx context.Context, ns, name string, sp *ServicePolicyReplace) (*ServicePolicy, error)
	DeleteServicePolicy(ctx context.Context, ns, name string) error
	ListServicePolicies(ctx context.Context, ns string) ([]*ServicePolicy, error)

	CreateRateLimiter(ctx context.Context, ns string, rl *XCRateLimiterCreate) (*XCRateLimiter, error)
	GetRateLimiter(ctx context.Context, ns, name string) (*XCRateLimiter, error)
	ReplaceRateLimiter(ctx context.Context, ns, name string, rl *XCRateLimiterReplace) (*XCRateLimiter, error)
	DeleteRateLimiter(ctx context.Context, ns, name string) error
	ListRateLimiters(ctx context.Context, ns string) ([]*XCRateLimiter, error)

	ClientNeedsUpdate(current, desired json.RawMessage) (bool, error)
}
```

Add `ClientNeedsUpdate` method to `Client` in `client.go` (append to file):

```go
func (c *Client) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	return NeedsUpdate(current, desired)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestClient_ImplementsXCClient
```

Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
make test
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/xcclient/interface.go internal/xcclient/client.go internal/xcclient/client_test.go
git commit -m "Add XCClient interface definition with compile-time conformance check"
```

---

### Task 18: Integration Tests (Retry, Concurrency, Error Injection)

**Files:**
- Create: `internal/xcclient/integration_test.go`

- [ ] **Step 1: Write integration tests using the fake server**

Create `internal/xcclient/integration_test.go`:

```go
package xcclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestIntegration_429RetryThenSuccess(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	fake.InjectError("POST", "origin_pools", "default", "", testutil.ErrorSpec{
		StatusCode: 429,
		Body:       "rate limited",
		Times:      2,
	})

	cfg := Config{TenantURL: fake.URL(), APIToken: "tok", MaxRetries: 5}
	client, err := NewClient(cfg, testLogr(), nil)
	require.NoError(t, err)
	client.baseDelay = 10 * time.Millisecond

	created, err := client.CreateOriginPool(context.Background(), "default", &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "retry-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 8080},
	})
	require.NoError(t, err)
	assert.Equal(t, "retry-pool", created.Metadata.Name)

	reqs := fake.Requests()
	postCount := 0
	for _, r := range reqs {
		if r.Method == "POST" {
			postCount++
		}
	}
	assert.Equal(t, 3, postCount)
}

func TestIntegration_409ConflictOnCreate(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateOriginPool(context.Background(), "default", &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "dup-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 8080},
	})

	_, err := client.CreateOriginPool(context.Background(), "default", &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "dup-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 9090},
	})
	assert.True(t, errors.Is(err, ErrConflict))
}

func TestIntegration_DeleteIdempotent(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	err := client.DeleteOriginPool(context.Background(), "default", "nonexistent")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestIntegration_CRUDLifecycle(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	// Create
	created, err := client.CreateOriginPool(context.Background(), "default", &OriginPoolCreate{
		Metadata: ObjectMeta{Name: "lifecycle-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 8080, LoadBalancerAlgorithm: "ROUND_ROBIN"},
	})
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-pool", created.Metadata.Name)

	// Get
	got, err := client.GetOriginPool(context.Background(), "default", "lifecycle-pool")
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-pool", got.Metadata.Name)

	// Replace
	replaced, err := client.ReplaceOriginPool(context.Background(), "default", "lifecycle-pool", &OriginPoolReplace{
		Metadata: ObjectMeta{Name: "lifecycle-pool", Namespace: "default"},
		Spec:     OriginPoolSpec{Port: 9090, LoadBalancerAlgorithm: "LEAST_REQUEST"},
	})
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-pool", replaced.Metadata.Name)

	// List
	list, err := client.ListOriginPools(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = client.DeleteOriginPool(context.Background(), "default", "lifecycle-pool")
	require.NoError(t, err)

	// Verify gone
	_, err = client.GetOriginPool(context.Background(), "default", "lifecycle-pool")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestIntegration_5xxNotRetried(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()

	fake.InjectError("GET", "origin_pools", "default", "test", testutil.ErrorSpec{
		StatusCode: 500,
		Body:       "internal error",
	})

	client := newTestClient(t, fake.URL())

	_, err := client.GetOriginPool(context.Background(), "default", "test")
	assert.True(t, errors.Is(err, ErrServerError))

	reqs := fake.Requests()
	assert.Len(t, reqs, 1)
}

func testLogr() logr.Logger {
	return logr.Discard()
}
```

Wait — we need to import logr. Add to imports:

```go
import (
	// ...
	"github.com/go-logr/logr"
	// ...
)
```

And the `testLogr()` function already returns `logr.Discard()`, so update the import.

- [ ] **Step 2: Run integration tests**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestIntegration
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/xcclient/integration_test.go
git commit -m "Add integration tests: retry, conflict, lifecycle, error injection"
```

---

### Task 19: Contract Tests (Real XC API)

**Files:**
- Create: `internal/xcclient/contract_test.go`

- [ ] **Step 1: Write contract tests gated by build tag**

Create `internal/xcclient/contract_test.go`:

```go
//go:build contract

package xcclient

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contractClient(t *testing.T) *Client {
	t.Helper()

	tenantURL := os.Getenv("XC_TENANT_URL")
	apiToken := os.Getenv("XC_API_TOKEN")

	if tenantURL == "" || apiToken == "" {
		t.Skip("XC_TENANT_URL and XC_API_TOKEN must be set for contract tests")
	}

	log := funcr.New(func(prefix, args string) {
		t.Logf("%s: %s", prefix, args)
	}, funcr.Options{Verbosity: 1})

	cfg := Config{
		TenantURL: tenantURL,
		APIToken:  apiToken,
	}
	client, err := NewClient(cfg, log, nil)
	require.NoError(t, err)
	return client
}

func contractNamespace(t *testing.T) string {
	t.Helper()
	ns := os.Getenv("XC_TEST_NAMESPACE")
	if ns == "" {
		ns = "operator-test"
	}
	return ns
}

func TestContract_OriginPool_CRUD(t *testing.T) {
	client := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()
	name := "contract-origin-pool"

	// Cleanup from prior runs
	client.DeleteOriginPool(ctx, ns, name)

	// Create
	created, err := client.CreateOriginPool(ctx, ns, &OriginPoolCreate{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Spec: OriginPoolSpec{
			Port:                  8080,
			LoadBalancerAlgorithm: "ROUND_ROBIN",
			OriginServers: []OriginServer{
				{PublicName: &PublicName{DNSName: "origin.example.com"}},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)
	assert.NotEmpty(t, created.SystemMetadata.UID)

	// Get
	got, err := client.GetOriginPool(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Metadata.Name)
	assert.NotEmpty(t, got.RawSpec)

	// Replace
	replaced, err := client.ReplaceOriginPool(ctx, ns, name, &OriginPoolReplace{
		Metadata: ObjectMeta{
			Name:            name,
			Namespace:       ns,
			ResourceVersion: got.Metadata.ResourceVersion,
		},
		Spec: OriginPoolSpec{
			Port:                  9090,
			LoadBalancerAlgorithm: "LEAST_REQUEST",
			OriginServers: []OriginServer{
				{PublicName: &PublicName{DNSName: "origin.example.com"}},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, replaced.Metadata.Name)

	// Verify change
	got2, err := client.GetOriginPool(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, "LEAST_REQUEST", got2.Spec.LoadBalancerAlgorithm)

	// List
	list, err := client.ListOriginPools(ctx, ns)
	require.NoError(t, err)
	found := false
	for _, p := range list {
		if p.Metadata.Name == name {
			found = true
		}
	}
	assert.True(t, found, "expected to find %s in list", name)

	// Delete
	err = client.DeleteOriginPool(ctx, ns, name)
	require.NoError(t, err)

	// Verify gone
	_, err = client.GetOriginPool(ctx, ns, name)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestContract_HealthCheck_CRUD(t *testing.T) {
	client := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()
	name := "contract-healthcheck"

	client.DeleteHealthCheck(ctx, ns, name)

	created, err := client.CreateHealthCheck(ctx, ns, &HealthCheckCreate{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Spec: HealthCheckSpec{
			HTTPHealthCheck:    &HTTPHealthCheck{Path: "/health"},
			HealthyThreshold:   2,
			UnhealthyThreshold: 5,
			Interval:           15,
			Timeout:            3,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	got, err := client.GetHealthCheck(ctx, ns, name)
	require.NoError(t, err)
	assert.Equal(t, name, got.Metadata.Name)

	err = client.DeleteHealthCheck(ctx, ns, name)
	require.NoError(t, err)

	_, err = client.GetHealthCheck(ctx, ns, name)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestContract_AppFirewall_CRUD(t *testing.T) {
	client := contractClient(t)
	ns := contractNamespace(t)
	ctx := context.Background()
	name := "contract-appfirewall"

	client.DeleteAppFirewall(ctx, ns, name)

	created, err := client.CreateAppFirewall(ctx, ns, &AppFirewallCreate{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Spec: AppFirewallSpec{
			DefaultDetectionSettings: []byte(`{}`),
			Monitoring:               []byte(`{}`),
			UseDefaultBlockingPage:   []byte(`{}`),
			AllowAllResponseCodes:    []byte(`{}`),
			DefaultAnonymization:     []byte(`{}`),
			DefaultBotSetting:        []byte(`{}`),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Metadata.Name)

	err = client.DeleteAppFirewall(ctx, ns, name)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Verify it compiles but is skipped in normal test runs**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v -run TestContract 2>&1 | head -5
```

Expected: no TestContract tests run (build tag not set).

```bash
go test ./internal/xcclient/ -v -tags=contract -run TestContract 2>&1 | head -5
```

Expected: tests run but skip due to missing env vars (`XC_TENANT_URL and XC_API_TOKEN must be set`).

- [ ] **Step 3: Commit**

```bash
git add internal/xcclient/contract_test.go
git commit -m "Add contract tests for real XC API (build tag: contract)"
```

---

### Task 20: Full Suite Verification + Tidy

**Files:**
- Modify: `go.mod` (tidy)

- [ ] **Step 1: Run go mod tidy**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go mod tidy
```

- [ ] **Step 2: Run full test suite**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
make test
```

Expected: all tests PASS. If any fail, fix before proceeding.

- [ ] **Step 3: Run linter**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
make lint
```

Expected: passes cleanly.

- [ ] **Step 4: Verify test count**

```bash
cd /Users/kevin/Projects/f5xc-k8s-operator
go test ./internal/xcclient/ -v 2>&1 | grep -c "--- PASS"
```

Expected: 30+ tests passing across all test files.

- [ ] **Step 5: Commit final tidy**

```bash
git add go.mod go.sum
git commit -m "Tidy Go module dependencies"
```
