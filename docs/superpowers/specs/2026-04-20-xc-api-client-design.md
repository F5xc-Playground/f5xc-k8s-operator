# F5 XC API Client — Design Spec

**Sub-project**: 1 of 5 (XC API Client)
**Date**: 2026-04-20
**Status**: Approved

## Overview

A hand-rolled Go client for the F5 Distributed Cloud REST API, covering 7 resource types needed by the Kubernetes operator. The client provides typed CRUD operations, dual authentication, per-endpoint rate limiting, change detection to avoid unnecessary writes, and optimistic concurrency handling.

This is the foundational dependency for all subsequent operator sub-projects.

## Scope

### In Scope

- Go client library at `internal/xcclient/`
- CRUD operations (Create, Get, Replace, Delete, List) for 7 resource types
- Dual authentication (API Token, P12 mTLS certificate)
- Per-endpoint rate limiting with configurable defaults
- Change detection (`NeedsUpdate`) to skip unnecessary PUT calls
- Unknown field preservation through GET → modify → PUT cycles
- Optimistic concurrency via conflict detection and retry
- 429 handling with exponential backoff (no Retry-After available)
- Typed error types for controller branching
- Prometheus metrics and structured logging via `logr`
- Unit tests, httptest integration tests, and contract tests against real XC API

### Out of Scope

- Additional XC resource types beyond the initial 7
- Webhook/event subscription (XC API doesn't support these)
- Cross-namespace list operations (XC API limitation)
- Kubernetes CRDs or controllers (sub-project 2+)
- Test cluster provisioning (handled separately)

## Resource Coverage

| Resource | XC API Path (plural) | Spec Complexity |
|----------|---------------------|-----------------|
| OriginPool | `origin_pools` | Origin server types (k8s, IP, FQDN, consul), health check refs, LB algorithm |
| HTTPLoadBalancer | `http_loadbalancers` | Most complex — 80+ fields, OneOf groups for WAF/bot/TLS/advertise, route arrays, origin pool refs with weights |
| TCPLoadBalancer | `tcp_loadbalancers` | TLS modes, origin pool refs, advertise config |
| AppFirewall | `app_firewalls` | Detection settings (presets vs custom), enforcement mode, threat campaigns toggle |
| HealthCheck | `healthchecks` | HTTP/HTTPS/TCP probes, interval, thresholds |
| ServicePolicy | `service_policys` | Irregular plural. Rule arrays with match conditions and actions |
| RateLimiter | `rate_limiters` | Threshold, burst, unit (second/minute/hour), max 8192 |

## Architecture

### Package Layout

```
internal/xcclient/
├── client.go          # Client struct, HTTP transport, auth, do() helper
├── interface.go       # XCClient interface definition
├── config.go          # Config struct, validation
├── errors.go          # Typed error types
├── ratelimit.go       # Per-endpoint rate limiter
├── metrics.go         # Prometheus metric definitions and recording
├── types.go           # Shared types (ObjectMeta, SystemMeta)
├── originpool.go      # OriginPool CRUD methods + types
├── httplb.go          # HTTPLoadBalancer CRUD methods + types
├── tcplb.go           # TCPLoadBalancer CRUD methods + types
├── appfirewall.go     # AppFirewall CRUD methods + types
├── healthcheck.go     # HealthCheck CRUD methods + types
├── servicepolicy.go   # ServicePolicy CRUD methods + types
├── ratelimiter.go     # RateLimiter CRUD methods + types (the XC resource, not our internal limiter)
├── diff.go            # NeedsUpdate / change detection logic
├── testutil/
│   └── fakeserver.go  # httptest-based fake XC API for integration tests
├── client_test.go     # Unit tests for transport, auth, retry
├── ratelimit_test.go  # Unit tests for rate limiter
├── diff_test.go       # Unit tests for change detection
├── originpool_test.go # Unit + httptest tests for OriginPool
├── httplb_test.go     # Unit + httptest tests for HTTPLoadBalancer
├── ...                # (one test file per resource)
└── contract_test.go   # Contract tests against real XC API (build tag: contract)
```

### Client Core

#### Configuration

```go
type Config struct {
    TenantURL    string
    APIToken     string         // Mutually exclusive with CertP12
    CertP12Path  string         // Mutually exclusive with APIToken
    CertPassword string
    RateLimits   RateLimitConfig
    HTTPTimeout  time.Duration  // Default: 30s
    MaxRetries   int            // Default: 5
}
```

Validation at construction time: exactly one of APIToken or CertP12Path must be set. TenantURL is required.

#### Authentication

- **API Token**: Adds `Authorization: APIToken <token>` header to every request.
- **P12 Certificate**: Loads P12 file at construction, configures `tls.Certificate` on the `http.Transport` for mTLS.

#### HTTP Transport

A private `do()` method handles the full request lifecycle:

1. Build URL: `{tenantURL}/api/config/namespaces/{ns}/{resourcePlural}[/{name}]`
2. Check per-endpoint rate limiter — block until token available
3. Set auth (header or TLS already configured on transport)
4. Execute HTTP request with timeout
5. Record Prometheus metrics
6. Map HTTP status to typed error or deserialize success response
7. On 429: exponential backoff with jitter, retry up to MaxRetries
8. On 409: return `ErrConflict`
9. On 404: return `ErrNotFound`
10. On 401/403: return `ErrAuth`
11. On 5xx: return `ErrServerError`

Resource path plurals are constants per resource type — not computed — to handle XC API's irregular pluralization (e.g., `service_policys`).

### Rate Limiting

#### Per-Endpoint Token Bucket

Each unique resource path gets its own `rate.Limiter` from `golang.org/x/time/rate`. Limiters are stored in a `sync.Map` for concurrent access.

```go
type RateLimitConfig struct {
    DefaultRPS   float64            // Default: 2 req/s
    DefaultBurst int                // Default: 5
    Overrides    map[string]EndpointLimit // Keyed by resource kind
}

type EndpointLimit struct {
    RPS   float64
    Burst int
}
```

No global aggregate limiter — XC limits are per-endpoint.

#### Change Detection

Before issuing a PUT, callers use `NeedsUpdate()` to compare current and desired state:

```go
func (c *Client) NeedsUpdate(resourceKind string, current, desired interface{}) (bool, error)
```

This is a pure comparison — no API calls. It:
- Strips server-managed fields (metadata.uid, system_metadata.*, status)
- Compares remaining spec fields using JSON normalization (marshal both sides, compare bytes)
- Returns false if nothing changed, preventing unnecessary PUTs

This is the primary defense against burning daily API limits.

#### 429 Backoff Strategy

Since XC 429 responses carry no Retry-After header:

| Retry | Delay | With Jitter (0-500ms) |
|-------|-------|-----------------------|
| 1 | 1s | 1.0s – 1.5s |
| 2 | 2s | 2.0s – 2.5s |
| 3 | 4s | 4.0s – 4.5s |
| 4 | 8s | 8.0s – 8.5s |
| 5 | 16s | 16.0s – 16.5s |

After 5 retries, return error to controller. Controller-runtime's own backoff handles further requeuing.

### Resource Type Structs

Each resource has three struct types:

- **CreateRequest** — fields for POST (metadata.name + spec)
- **ReplaceRequest** — fields for PUT (metadata + spec)
- **Object** — full response (metadata + system_metadata + spec)

We model only the fields the operator manages. To prevent data loss on PUT, each Object includes a `RawSpec json.RawMessage` field that captures the full spec JSON from GET. On Replace, managed fields are overlaid onto the raw spec before serialization. This preserves fields the client doesn't model.

### XCClient Interface

```go
type XCClient interface {
    // Origin Pools
    CreateOriginPool(ctx context.Context, ns string, pool *OriginPoolCreate) (*OriginPool, error)
    GetOriginPool(ctx context.Context, ns, name string) (*OriginPool, error)
    ReplaceOriginPool(ctx context.Context, ns, name string, pool *OriginPoolReplace) (*OriginPool, error)
    DeleteOriginPool(ctx context.Context, ns, name string) error
    ListOriginPools(ctx context.Context, ns string) ([]*OriginPool, error)

    // HTTPLoadBalancer
    CreateHTTPLoadBalancer(ctx context.Context, ns string, lb *HTTPLoadBalancerCreate) (*HTTPLoadBalancer, error)
    GetHTTPLoadBalancer(ctx context.Context, ns, name string) (*HTTPLoadBalancer, error)
    ReplaceHTTPLoadBalancer(ctx context.Context, ns, name string, lb *HTTPLoadBalancerReplace) (*HTTPLoadBalancer, error)
    DeleteHTTPLoadBalancer(ctx context.Context, ns, name string) error
    ListHTTPLoadBalancers(ctx context.Context, ns string) ([]*HTTPLoadBalancer, error)

    // TCPLoadBalancer
    CreateTCPLoadBalancer(ctx context.Context, ns string, lb *TCPLoadBalancerCreate) (*TCPLoadBalancer, error)
    GetTCPLoadBalancer(ctx context.Context, ns, name string) (*TCPLoadBalancer, error)
    ReplaceTCPLoadBalancer(ctx context.Context, ns, name string, lb *TCPLoadBalancerReplace) (*TCPLoadBalancer, error)
    DeleteTCPLoadBalancer(ctx context.Context, ns, name string) error
    ListTCPLoadBalancers(ctx context.Context, ns string) ([]*TCPLoadBalancer, error)

    // AppFirewall
    CreateAppFirewall(ctx context.Context, ns string, fw *AppFirewallCreate) (*AppFirewall, error)
    GetAppFirewall(ctx context.Context, ns, name string) (*AppFirewall, error)
    ReplaceAppFirewall(ctx context.Context, ns, name string, fw *AppFirewallReplace) (*AppFirewall, error)
    DeleteAppFirewall(ctx context.Context, ns, name string) error
    ListAppFirewalls(ctx context.Context, ns string) ([]*AppFirewall, error)

    // HealthCheck
    CreateHealthCheck(ctx context.Context, ns string, hc *HealthCheckCreate) (*HealthCheck, error)
    GetHealthCheck(ctx context.Context, ns, name string) (*HealthCheck, error)
    ReplaceHealthCheck(ctx context.Context, ns, name string, hc *HealthCheckReplace) (*HealthCheck, error)
    DeleteHealthCheck(ctx context.Context, ns, name string) error
    ListHealthChecks(ctx context.Context, ns string) ([]*HealthCheck, error)

    // ServicePolicy
    CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error)
    GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error)
    ReplaceServicePolicy(ctx context.Context, ns, name string, sp *ServicePolicyReplace) (*ServicePolicy, error)
    DeleteServicePolicy(ctx context.Context, ns, name string) error
    ListServicePolicies(ctx context.Context, ns string) ([]*ServicePolicy, error)

    // RateLimiter
    CreateRateLimiter(ctx context.Context, ns string, rl *RateLimiterCreate) (*RateLimiter, error)
    GetRateLimiter(ctx context.Context, ns, name string) (*RateLimiter, error)
    ReplaceRateLimiter(ctx context.Context, ns, name string, rl *RateLimiterReplace) (*RateLimiter, error)
    DeleteRateLimiter(ctx context.Context, ns, name string) error
    ListRateLimiters(ctx context.Context, ns string) ([]*RateLimiter, error)

    // Change detection
    NeedsUpdate(resourceKind string, current, desired interface{}) (bool, error)
}
```

Mocks are auto-generated via `go generate` with `counterfeiter` or `mockgen`.

### Error Types

```go
var (
    ErrNotFound    = errors.New("xc: resource not found")           // 404
    ErrConflict    = errors.New("xc: resource conflict")            // 409
    ErrRateLimited = errors.New("xc: rate limited")                 // 429
    ErrServerError = errors.New("xc: server error")                 // 5xx
    ErrAuth        = errors.New("xc: authentication failed")        // 401/403
)
```

All errors wrap the sentinel with additional context (HTTP status, endpoint, response body excerpt). Controllers use `errors.Is()` to branch:

| Error | Controller Behavior |
|-------|-------------------|
| `ErrNotFound` on GET | Create the resource |
| `ErrNotFound` on DELETE | Treat as success (idempotent) |
| `ErrConflict` | Re-GET the object and retry update |
| `ErrRateLimited` | Requeue (controller-runtime backoff) |
| `ErrAuth` | Log error, set Degraded status condition, don't retry |
| `ErrServerError` | Requeue with delay |

### Observability

#### Prometheus Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `f5xc_api_requests_total` | Counter | `endpoint`, `method`, `status_code` |
| `f5xc_api_request_duration_seconds` | Histogram | `endpoint`, `method` |
| `f5xc_api_rate_limit_hits_total` | Counter | `endpoint` |
| `f5xc_api_retries_total` | Counter | `endpoint`, `reason` |
| `f5xc_api_updates_skipped_total` | Counter | `endpoint` |

All labels are low-cardinality. No namespace, object name, or UID labels.

#### Logging

Structured logging via `logr` (controller-runtime standard):

| Level | What |
|-------|------|
| Debug (V=1) | Request/response round-trips: method, path, status, duration |
| Info | Successful creates/deletes, updates skipped by change detection |
| Warning | 429s received, retries triggered |
| Error | Final failures after retry exhaustion, auth failures |

## Testing Strategy

### Unit Tests

No network, no external dependencies. Test:

- URL construction including irregular plurals (`service_policys`)
- Auth configuration: API token header, P12 TLS, both-set error, neither-set error
- Change detection: `NeedsUpdate` ignores server-managed fields, detects real spec changes, handles unknown field preservation
- Error type mapping: HTTP status codes → typed errors
- Rate limiter: per-endpoint isolation, burst behavior, config overrides
- JSON serialization: Go structs round-trip correctly, unknown fields preserved

Run with: `make test`

### Integration Tests (httptest)

A reusable fake XC API server (`internal/xcclient/testutil/fakeserver.go`) that:

- Responds to correct resource paths with realistic JSON
- Simulates 429s, 409s, 5xx errors on demand
- Tracks received requests for assertion
- Validates auth headers/TLS

Tests verify:

- Full request/response flow with auth
- Retry behavior on 429 (verify exponential backoff timing)
- Optimistic concurrency: 409 → re-GET → retry
- Rate limiter integration with real HTTP calls
- Change detection preventing unnecessary PUTs

Run with: `make test`

### Contract Tests (Real XC API)

Run against a real F5 XC tenant from a real Kubernetes cluster.

- Full CRUD lifecycle per resource type: create → get → replace → get → verify → delete → verify gone
- Dedicated XC namespace (`operator-test`) for isolation
- Test both API Token and P12 Certificate auth
- Observe actual rate limit behavior
- Gated behind build tag: `//go:build contract`
- Requires `XC_TENANT_URL` and credentials in environment

Run with: `make test-contract`

## XC API Gotchas

Documented here as reference for implementation:

1. **No PATCH** — always GET → modify → PUT the entire object
2. **Irregular plurals** — `service_policys`, `discoverys`; hardcode as constants, never compute
3. **No cross-namespace list** — must enumerate namespaces individually
4. **OneOf fields** — setting one side implicitly disables the other; must be explicit in serialization
5. **Views auto-create children** — we use the views API (e.g., `views.http_loadbalancer`), not the raw objects
6. **Rate limits are per-endpoint** — no aggregate limit, but daily limits exist
7. **429s have no Retry-After** — must implement our own backoff
8. **PUT replaces entirely** — must preserve all fields, including those we don't model

## Dependencies

- `golang.org/x/time/rate` — token bucket rate limiter
- `github.com/prometheus/client_golang` — metrics
- `github.com/go-logr/logr` — structured logging (controller-runtime standard)
- Standard library: `net/http`, `crypto/tls`, `encoding/json`, `crypto/x509`
- Test: `net/http/httptest`, mock generator (counterfeiter or mockgen)

## Subsequent Sub-Projects

This client is consumed by:

- **Sub-project 2**: Operator Core + OriginPool CRD (first end-to-end reconciliation)
- **Sub-project 3**: HTTP Load Balancer + Security CRDs
- **Sub-project 4**: Ingress Controller Integration
- **Sub-project 5**: Gateway API Integration
