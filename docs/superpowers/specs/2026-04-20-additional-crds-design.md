# Additional CRDs — Design Spec

**Sub-project**: 3 of 5 (HTTP Load Balancer + Security CRDs)
**Date**: 2026-04-20
**Status**: Draft

## Overview

Add CRDs and controllers for the remaining 6 F5 XC resource types: HTTPLoadBalancer, TCPLoadBalancer, AppFirewall, HealthCheck, ServicePolicy, and RateLimiter. Each follows the same reconciliation pattern established by the OriginPool controller in sub-project 2. No new shared infrastructure — just per-resource types, mappers, controllers, and tests.

## Scope

### In Scope

- 6 new CRDs in API group `xc.f5.com/v1alpha1`
- 6 new reconcilers, each following the OriginPool Get→Compare→Act pattern
- 6 new mappers translating K8s CRD spec to XC client types
- Full fidelity: every XC API field present in the CRD, required fields enforced, everything else optional
- Per-resource: mapper unit tests, controller unit tests (envtest + fake XC client), integration tests (envtest + FakeXCServer), contract test scaffolds
- Extract shared constants from `originpool_types.go` to `constants.go`
- Register all controllers in `cmd/main.go`
- Generated CRD YAML, RBAC, sample CRs

### Out of Scope

- Generic/shared reconciler abstraction (each controller is self-contained)
- Changes to the XC client (`internal/xcclient/`)
- Changes to the existing OriginPool controller logic
- Ingress controller or Gateway API integration (sub-projects 4–5)

## File Structure

### New Files

| Resource | Types | Mapper | Controller | Mapper Tests | Controller Tests | Integration Tests |
|---|---|---|---|---|---|---|
| HTTPLoadBalancer | `api/v1alpha1/httplb_types.go` | `internal/controller/httplb_mapper.go` | `internal/controller/httplb_controller.go` | `httplb_mapper_test.go` | `httplb_controller_test.go` | `httplb_integration_test.go` |
| TCPLoadBalancer | `api/v1alpha1/tcplb_types.go` | `internal/controller/tcplb_mapper.go` | `internal/controller/tcplb_controller.go` | `tcplb_mapper_test.go` | `tcplb_controller_test.go` | `tcplb_integration_test.go` |
| AppFirewall | `api/v1alpha1/appfirewall_types.go` | `internal/controller/appfirewall_mapper.go` | `internal/controller/appfirewall_controller.go` | `appfirewall_mapper_test.go` | `appfirewall_controller_test.go` | `appfirewall_integration_test.go` |
| HealthCheck | `api/v1alpha1/healthcheck_types.go` | `internal/controller/healthcheck_mapper.go` | `internal/controller/healthcheck_controller.go` | `healthcheck_mapper_test.go` | `healthcheck_controller_test.go` | `healthcheck_integration_test.go` |
| ServicePolicy | `api/v1alpha1/servicepolicy_types.go` | `internal/controller/servicepolicy_mapper.go` | `internal/controller/servicepolicy_controller.go` | `servicepolicy_mapper_test.go` | `servicepolicy_controller_test.go` | `servicepolicy_integration_test.go` |
| RateLimiter | `api/v1alpha1/ratelimiter_types.go` | `internal/controller/ratelimiter_mapper.go` | `internal/controller/ratelimiter_controller.go` | `ratelimiter_mapper_test.go` | `ratelimiter_controller_test.go` | `ratelimiter_integration_test.go` |

### New Shared File

- `api/v1alpha1/constants.go` — Shared constants extracted from `originpool_types.go`

### Modified Files

- `api/v1alpha1/originpool_types.go` — Remove constants (moved to `constants.go`)
- `cmd/main.go` — Register 6 new reconcilers
- `internal/controller/contract_test.go` — Add 6 new contract test functions
- `api/v1alpha1/zz_generated.deepcopy.go` — Regenerated
- `config/crd/bases/` — 6 new generated CRD YAML files
- `config/rbac/role.yaml` — Regenerated with new resource permissions

### New Sample CRs

- `config/samples/httplb.yaml`
- `config/samples/tcplb.yaml`
- `config/samples/appfirewall.yaml`
- `config/samples/healthcheck.yaml`
- `config/samples/servicepolicy.yaml`
- `config/samples/ratelimiter.yaml`

## CRD Type Definitions

All types use `camelCase` JSON tags (K8s convention). The mapper translates to `snake_case` for the XC API. OneOf groups use `*apiextensionsv1.JSON` for raw passthrough, matching the OriginPool pattern for `useTLS`/`noTLS`.

All CRDs share the same status shape, kubebuilder markers (root, status subresource, print columns for Ready/Synced/Age), and `init()` registration pattern.

### Shared Status (used by all 6 CRDs)

```go
type {Resource}Status struct {
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
    XCUID              string             `json:"xcUID,omitempty"`
    XCNamespace        string             `json:"xcNamespace,omitempty"`
}
```

Each resource defines its own `{Resource}Status` type (not a shared type) for controller-runtime compatibility, but the fields are identical.

### HTTPLoadBalancer (short name: `hlb`)

The most complex CRD. Mirrors the full `xcclient.HTTPLoadBalancerSpec`.

```go
type HTTPLoadBalancerSpec struct {
    // Required
    Domains           []string   `json:"domains"`
    DefaultRoutePools []RoutePool `json:"defaultRoutePools"`

    // Optional typed
    Routes            []apiextensionsv1.JSON `json:"routes,omitempty"`

    // TLS OneOf
    HTTP              *apiextensionsv1.JSON `json:"http,omitempty"`
    HTTPS             *apiextensionsv1.JSON `json:"https,omitempty"`
    HTTPSAutoCert     *apiextensionsv1.JSON `json:"httpsAutoCert,omitempty"`

    // WAF OneOf
    DisableWAF        *apiextensionsv1.JSON `json:"disableWAF,omitempty"`
    AppFirewall       *ObjectRef            `json:"appFirewall,omitempty"`

    // Bot defense OneOf
    DisableBotDefense *apiextensionsv1.JSON `json:"disableBotDefense,omitempty"`
    BotDefense        *apiextensionsv1.JSON `json:"botDefense,omitempty"`

    // API discovery OneOf
    DisableAPIDiscovery *apiextensionsv1.JSON `json:"disableAPIDiscovery,omitempty"`
    EnableAPIDiscovery  *apiextensionsv1.JSON `json:"enableAPIDiscovery,omitempty"`

    // IP reputation OneOf
    DisableIPReputation *apiextensionsv1.JSON `json:"disableIPReputation,omitempty"`
    EnableIPReputation  *apiextensionsv1.JSON `json:"enableIPReputation,omitempty"`

    // Rate limit OneOf
    DisableRateLimit *apiextensionsv1.JSON `json:"disableRateLimit,omitempty"`
    RateLimit        *apiextensionsv1.JSON `json:"rateLimit,omitempty"`

    // Challenge OneOf
    NoChallenge          *apiextensionsv1.JSON `json:"noChallenge,omitempty"`
    JSChallenge          *apiextensionsv1.JSON `json:"jsChallenge,omitempty"`
    CaptchaChallenge     *apiextensionsv1.JSON `json:"captchaChallenge,omitempty"`
    PolicyBasedChallenge *apiextensionsv1.JSON `json:"policyBasedChallenge,omitempty"`

    // LB algorithm OneOf
    RoundRobin         *apiextensionsv1.JSON `json:"roundRobin,omitempty"`
    LeastActive        *apiextensionsv1.JSON `json:"leastActive,omitempty"`
    Random             *apiextensionsv1.JSON `json:"random,omitempty"`
    SourceIPStickiness *apiextensionsv1.JSON `json:"sourceIPStickiness,omitempty"`
    CookieStickiness   *apiextensionsv1.JSON `json:"cookieStickiness,omitempty"`
    RingHash           *apiextensionsv1.JSON `json:"ringHash,omitempty"`

    // Advertise OneOf
    AdvertiseOnPublicDefaultVIP *apiextensionsv1.JSON `json:"advertiseOnPublicDefaultVIP,omitempty"`
    AdvertiseOnPublic           *apiextensionsv1.JSON `json:"advertiseOnPublic,omitempty"`
    AdvertiseCustom             *apiextensionsv1.JSON `json:"advertiseCustom,omitempty"`
    DoNotAdvertise              *apiextensionsv1.JSON `json:"doNotAdvertise,omitempty"`

    // Service policies OneOf
    ServicePoliciesFromNamespace *apiextensionsv1.JSON `json:"servicePoliciesFromNamespace,omitempty"`
    ActiveServicePolicies        *apiextensionsv1.JSON `json:"activeServicePolicies,omitempty"`
    NoServicePolicies            *apiextensionsv1.JSON `json:"noServicePolicies,omitempty"`

    // User ID OneOf
    UserIDClientIP *apiextensionsv1.JSON `json:"userIDClientIP,omitempty"`
}
```

`RoutePool` is a new shared type (used by both HTTPLoadBalancer and TCPLoadBalancer):

```go
type RoutePool struct {
    Pool     ObjectRef `json:"pool"`
    Weight   *uint32   `json:"weight,omitempty"`
    Priority *uint32   `json:"priority,omitempty"`
}
```

`ObjectRef` already exists in `originpool_types.go` and will remain there (or move to `constants.go` alongside the shared constants).

### TCPLoadBalancer (short name: `tlb`)

```go
type TCPLoadBalancerSpec struct {
    // Required
    Domains     []string    `json:"domains"`
    ListenPort  uint32      `json:"listenPort"`
    OriginPools []RoutePool `json:"originPools"`

    // TLS OneOf
    NoTLS          *apiextensionsv1.JSON `json:"noTLS,omitempty"`
    TLSParameters  *apiextensionsv1.JSON `json:"tlsParameters,omitempty"`
    TLSPassthrough *apiextensionsv1.JSON `json:"tlsPassthrough,omitempty"`

    // Advertise OneOf
    AdvertiseOnPublicDefaultVIP *apiextensionsv1.JSON `json:"advertiseOnPublicDefaultVIP,omitempty"`
    AdvertiseOnPublic           *apiextensionsv1.JSON `json:"advertiseOnPublic,omitempty"`
    AdvertiseCustom             *apiextensionsv1.JSON `json:"advertiseCustom,omitempty"`
    DoNotAdvertise              *apiextensionsv1.JSON `json:"doNotAdvertise,omitempty"`
}
```

### AppFirewall (short name: `afw`)

All fields optional — XC API provides sensible defaults.

```go
type AppFirewallSpec struct {
    // Detection OneOf
    DefaultDetectionSettings *apiextensionsv1.JSON `json:"defaultDetectionSettings,omitempty"`
    DetectionSettings        *apiextensionsv1.JSON `json:"detectionSettings,omitempty"`

    // Enforcement mode OneOf
    Monitoring *apiextensionsv1.JSON `json:"monitoring,omitempty"`
    Blocking   *apiextensionsv1.JSON `json:"blocking,omitempty"`

    // Blocking page OneOf
    UseDefaultBlockingPage *apiextensionsv1.JSON `json:"useDefaultBlockingPage,omitempty"`
    BlockingPage           *apiextensionsv1.JSON `json:"blockingPage,omitempty"`

    // Response codes OneOf
    AllowAllResponseCodes *apiextensionsv1.JSON `json:"allowAllResponseCodes,omitempty"`
    AllowedResponseCodes  *apiextensionsv1.JSON `json:"allowedResponseCodes,omitempty"`

    // Bot setting OneOf
    DefaultBotSetting    *apiextensionsv1.JSON `json:"defaultBotSetting,omitempty"`
    BotProtectionSetting *apiextensionsv1.JSON `json:"botProtectionSetting,omitempty"`

    // Anonymization OneOf
    DefaultAnonymization *apiextensionsv1.JSON `json:"defaultAnonymization,omitempty"`
    CustomAnonymization  *apiextensionsv1.JSON `json:"customAnonymization,omitempty"`

    // Loadbalancer setting
    UseLoadbalancerSetting *apiextensionsv1.JSON `json:"useLoadbalancerSetting,omitempty"`
}
```

### HealthCheck (short name: `hc`)

Uses typed structs for the health check probe config (not raw JSON) since the fields are simple and well-defined.

```go
type HealthCheckSpec struct {
    // Probe type OneOf
    HTTPHealthCheck *HTTPHealthCheckSpec `json:"httpHealthCheck,omitempty"`
    TCPHealthCheck  *TCPHealthCheckSpec  `json:"tcpHealthCheck,omitempty"`

    // Thresholds and timing (all optional, XC has defaults)
    HealthyThreshold   *uint32 `json:"healthyThreshold,omitempty"`
    UnhealthyThreshold *uint32 `json:"unhealthyThreshold,omitempty"`
    Interval           *uint32 `json:"interval,omitempty"`
    Timeout            *uint32 `json:"timeout,omitempty"`
    JitterPercent      *uint32 `json:"jitterPercent,omitempty"`
}

type HTTPHealthCheckSpec struct {
    Path                string   `json:"path,omitempty"`
    UseHTTP2            bool     `json:"useHTTP2,omitempty"`
    ExpectedStatusCodes []string `json:"expectedStatusCodes,omitempty"`
}

type TCPHealthCheckSpec struct {
    Send    string `json:"send,omitempty"`
    Receive string `json:"receive,omitempty"`
}
```

### ServicePolicy (short name: `sp`)

```go
type ServicePolicySpec struct {
    Algo  string                  `json:"algo,omitempty"`
    Rules []apiextensionsv1.JSON  `json:"rules,omitempty"`
}
```

### RateLimiter (short name: `rl`)

```go
type RateLimiterSpec struct {
    // Required
    Threshold uint32 `json:"threshold"`
    Unit      string `json:"unit"`

    // Optional
    BurstMultiplier *uint32 `json:"burstMultiplier,omitempty"`
}
```

## Mapper Pattern

Each resource gets 3 builder functions and 1 spec mapper, following `originpool_mapper.go` exactly:

```go
func build{Resource}Create(cr *v1alpha1.{Resource}, xcNamespace string) *xcclient.{Resource}Create
func build{Resource}Replace(cr *v1alpha1.{Resource}, xcNamespace, resourceVersion string) *xcclient.{Resource}Replace
func build{Resource}DesiredSpecJSON(cr *v1alpha1.{Resource}, xcNamespace string) (json.RawMessage, error)
func map{Resource}Spec(spec *v1alpha1.{Resource}Spec) xcclient.{Resource}Spec
```

The naming convention for the desired spec JSON builder is `build{Resource}DesiredSpecJSON` (prefixed with resource name) to avoid collisions since all controllers share the `controller` package. The existing OriginPool function `buildDesiredSpecJSON` will be renamed to `buildOriginPoolDesiredSpecJSON` for consistency.

### OneOf Passthrough

All `*apiextensionsv1.JSON` fields map to `json.RawMessage` in the XC client spec:

```go
if spec.HTTPS != nil {
    out.HTTPS = json.RawMessage(spec.HTTPS.Raw)
}
```

### Typed Struct Mapping

Fields with typed K8s structs (e.g., `HTTPHealthCheckSpec`, `RoutePool`, `ObjectRef`) get dedicated mapper functions:

```go
func mapRoutePool(rp *v1alpha1.RoutePool) xcclient.RoutePool
func mapHTTPHealthCheck(hc *v1alpha1.HTTPHealthCheckSpec) *xcclient.HTTPHealthCheck
func mapTCPHealthCheck(hc *v1alpha1.TCPHealthCheckSpec) *xcclient.TCPHealthCheck
```

### XC Client API Quirks

**Value receivers:** Two resources use value receivers instead of pointers in the XC client:
- `CreateHealthCheck(ctx, ns, hc CreateHealthCheck)` — value, not `*CreateHealthCheck`
- `CreateRateLimiter(ctx, ns, rl XCRateLimiterCreate)` — value, not `*XCRateLimiterCreate`

The mappers for these resources return values instead of pointers to match.

**HealthCheck response type:** The `xcclient.HealthCheck` response type stores the spec as `RawSpec json.RawMessage` with JSON tag `json:"spec"` — it does not have a parsed `Spec` field like the other resources. The controller uses `RawSpec` directly for `ClientNeedsUpdate` comparison, which works the same as other resources. The `setStatus` function uses `Metadata.ResourceVersion` and `SystemMetadata.UID` which are available on the response regardless.

## Controller Pattern

Each controller is a copy of `OriginPoolReconciler` with resource-specific types and XC client methods substituted. The structure is identical:

```go
type {Resource}Reconciler struct {
    client.Client
    Log       logr.Logger
    ClientSet *xcclientset.ClientSet
}
```

### Reconcile Loop

Same Get→Compare→Act pattern:

1. Fetch K8s CR
2. Handle deletion (finalizer + orphan annotation)
3. Add finalizer if missing
4. Resolve XC namespace from annotation or K8s namespace
5. `xc.Get{Resource}()` — branch on `ErrNotFound`
6. Not found → `handleCreate` using `build{Resource}Create`
7. Found → `build{Resource}DesiredSpecJSON` → `ClientNeedsUpdate` → `handleUpdate` if changed
8. `handleXCError` with identical retry logic

### RBAC Markers

Each controller declares its own RBAC markers:

```go
// +kubebuilder:rbac:groups=xc.f5.com,resources=httploadbalancers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=httploadbalancers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=httploadbalancers/finalizers,verbs=update
```

### SetupWithManager

Each controller watches only its own resource type:

```go
func (r *{Resource}Reconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.{Resource}{}).
        Complete(r)
}
```

### Helper Function Naming

Since all controllers live in package `controller`, helper functions are prefixed with the resource name to avoid collisions:
- `resolve{Resource}XCNamespace(cr)` — though these all do the same thing (check annotation, fallback to K8s namespace), each controller has its own typed version since the CR type parameter differs.
- `setStatus`, `setCondition`, `handleXCError`, `operationFailReason` — same logic, different CR types. Each controller defines its own copies.

## Registration in main.go

Six new reconciler registrations in `cmd/main.go`, identical pattern to OriginPool:

```go
if err := (&controller.HTTPLoadBalancerReconciler{
    Client:    mgr.GetClient(),
    Log:       ctrl.Log.WithName("controllers").WithName("HTTPLoadBalancer"),
    ClientSet: cs,
}).SetupWithManager(mgr); err != nil {
    log.Error(err, "unable to create controller", "controller", "HTTPLoadBalancer")
    os.Exit(1)
}
// ... repeat for TCPLoadBalancer, AppFirewall, HealthCheck, ServicePolicy, RateLimiter
```

## Constants Extraction

Move from `originpool_types.go` to `api/v1alpha1/constants.go`:

```go
const (
    FinalizerXCCleanup       = "xc.f5.com/cleanup"
    AnnotationXCNamespace    = "f5xc.io/namespace"
    AnnotationDeletionPolicy = "f5xc.io/deletion-policy"
    DeletionPolicyOrphan     = "orphan"
    ConditionReady           = "Ready"
    ConditionSynced          = "Synced"
    ReasonCreateSucceeded    = "CreateSucceeded"
    ReasonUpdateSucceeded    = "UpdateSucceeded"
    ReasonUpToDate           = "UpToDate"
    ReasonDeleteSucceeded    = "DeleteSucceeded"
    ReasonCreateFailed       = "CreateFailed"
    ReasonUpdateFailed       = "UpdateFailed"
    ReasonDeleteFailed       = "DeleteFailed"
    ReasonAuthFailure        = "AuthFailure"
    ReasonRateLimited        = "RateLimited"
    ReasonServerError        = "ServerError"
    ReasonConflict           = "Conflict"
)
```

`ObjectRef` and `RoutePool` types move to a new `api/v1alpha1/shared_types.go` since they're used by multiple CRDs. `ObjectRef` is currently defined in `originpool_types.go` and will be removed from there. `RoutePool` is new (used by HTTPLoadBalancer and TCPLoadBalancer).

## Testing

### Mapper Unit Tests

Per-resource file in `internal/controller/`. Tests verify:

- Required fields mapped correctly
- Optional fields omitted when nil
- OneOf fields pass through raw JSON correctly
- `build{Resource}DesiredSpecJSON` produces JSON matching what `ClientNeedsUpdate` expects
- Typed nested structs (RoutePool, HTTPHealthCheckSpec, etc.) mapped correctly

### Controller Unit Tests (envtest + fake XC client)

Per-resource file in `internal/controller/`. Each defines its own `fake{Resource}XCClient` implementing the full `XCClient` interface, with configurable responses for the relevant resource methods. Same 7 test cases as OriginPool:

1. **CreateWhenNotFound** — Get returns ErrNotFound, verify Create called, status set
2. **SkipUpdateWhenUpToDate** — Get returns matching object, verify no Replace call
3. **UpdateWhenChanged** — Get returns stale object, verify Replace called
4. **AuthFailureNoRequeue** — Get returns ErrAuth, verify no requeue, status set
5. **DeletionCallsXCDelete** — CR has deletion timestamp, verify Delete called, finalizer removed
6. **DeletionOrphanPolicy** — CR has orphan annotation, verify Delete NOT called
7. **XCNamespaceAnnotation** — CR has namespace annotation, verify XC operations use annotated namespace

Uses the existing `suite_test.go` setup (shared `setupSuite`, `startManager`).

### Integration Tests (envtest + FakeXCServer)

Per-resource file in `internal/controller/`. Same 4 test cases:

1. **CreateLifecycle** — Create CR, verify XC object created via FakeXCServer
2. **DeleteLifecycle** — Delete CR, verify XC object deleted
3. **ErrorInjection429** — Inject 429 error, verify requeue after 60s
4. **ErrorInjection401** — Inject 401 error, verify no requeue, status shows auth failure

Uses `newRealClient(t, serverURL)` helper and `FakeXCServer` from `internal/xcclient/testutil/`.

### Contract Tests

Appended to existing `internal/controller/contract_test.go`. One function per resource, `//go:build contract` gated, skips without `XC_TENANT_URL` and `XC_API_TOKEN`. Full CRUD lifecycle: create → get → update → get (verify changed) → delete → get (verify 404).

## Sample CRs

One minimal example per resource in `config/samples/`, showing only required fields.

### config/samples/httplb.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: example-httplb
spec:
  domains:
    - "app.example.com"
  defaultRoutePools:
    - pool:
        name: example-pool
      weight: 1
  advertiseOnPublicDefaultVIP: {}
```

### config/samples/tcplb.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: TCPLoadBalancer
metadata:
  name: example-tcplb
spec:
  domains:
    - "tcp.example.com"
  listenPort: 443
  originPools:
    - pool:
        name: example-pool
      weight: 1
```

### config/samples/appfirewall.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: AppFirewall
metadata:
  name: example-appfirewall
spec:
  blocking: {}
```

### config/samples/healthcheck.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HealthCheck
metadata:
  name: example-healthcheck
spec:
  httpHealthCheck:
    path: "/healthz"
```

### config/samples/servicepolicy.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: ServicePolicy
metadata:
  name: example-servicepolicy
spec:
  algo: "FIRST_MATCH"
```

### config/samples/ratelimiter.yaml
```yaml
apiVersion: xc.f5.com/v1alpha1
kind: RateLimiter
metadata:
  name: example-ratelimiter
spec:
  threshold: 100
  unit: "MINUTE"
```
