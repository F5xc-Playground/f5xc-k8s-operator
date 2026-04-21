# Origin Discovery Design Spec

## Goal

Extend the OriginPool CRD to support dynamic origin server discovery from Kubernetes resources. Instead of requiring users to manually specify static IPs or FQDNs, the controller can watch an existing Service, Ingress, Gateway, or OpenShift Route and automatically resolve the external address and port for use as an XC origin server.

This operator is the XC delivery layer — it does not replace or act as an ingress controller. It reads from existing cluster exposure mechanisms to configure F5 XC origin pools. This follows the same pattern used by CDN/edge vendors like Cloudflare Tunnel and Fastly's community operators.

## Scope

### In Scope

- New `discover` OneOf field on `OriginServer` for dynamic address resolution
- Resolution from: Service (LoadBalancer, NodePort, ExternalName, externalIPs), Ingress, Gateway, OpenShift Route
- Address and port override fields for NAT and custom routing scenarios
- Per-origin discovery status reporting in OriginPoolStatus
- Dynamic watches on referenced K8s resources (re-reconcile when they change)
- Conditional watch registration (Gateway API and OpenShift Route CRDs only watched if installed)
- Full test coverage: resolver unit tests, controller tests, integration tests

### Out of Scope

- Changes to the XC client (`internal/xcclient/`)
- Changes to other CRD controllers (HTTPLoadBalancer, etc.)
- New CRDs — this extends the existing OriginPool only
- Webhook validation (can be added later)
- Multi-port discovery (resolves first/primary port; override available for others)

## Data Model

### New Types

#### OriginServerDiscover

Added as a new OneOf option on `OriginServer`, alongside `publicIP`, `publicName`, `privateIP`, `privateName`, `k8sService`, `consulService`:

```go
type OriginServer struct {
    PublicIP      *PublicIP               `json:"publicIP,omitempty"`
    PublicName    *PublicName             `json:"publicName,omitempty"`
    PrivateIP     *PrivateIP              `json:"privateIP,omitempty"`
    PrivateName   *PrivateName            `json:"privateName,omitempty"`
    K8SService    *K8SService             `json:"k8sService,omitempty"`
    ConsulService *ConsulService          `json:"consulService,omitempty"`
    Discover      *OriginServerDiscover   `json:"discover,omitempty"`  // NEW
}

type OriginServerDiscover struct {
    Resource        ResourceRef `json:"resource"`
    AddressOverride string      `json:"addressOverride,omitempty"`
    PortOverride    *uint32     `json:"portOverride,omitempty"`
}
```

#### ResourceRef

Added to `shared_types.go` for reuse:

```go
type ResourceRef struct {
    Kind      string `json:"kind"`
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`
}
```

The `Kind` field determines the API group implicitly:

| Kind | API Group | Resource |
|------|-----------|----------|
| `Service` | `""` (core) | `v1/Service` |
| `Ingress` | `networking.k8s.io` | `v1/Ingress` |
| `Gateway` | `gateway.networking.k8s.io` | `v1/Gateway` |
| `Route` | `route.openshift.io` | `v1/Route` |

No `apiGroup` field is exposed to users — the controller infers it from Kind and rejects unsupported values. If a future resource type creates an ambiguity, `apiGroup` can be added as an optional disambiguator.

When `namespace` is omitted, it defaults to the OriginPool CR's namespace.

#### DiscoveredOrigin (Status)

Added to `OriginPoolStatus`:

```go
type OriginPoolStatus struct {
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
    XCUID              string             `json:"xcUID,omitempty"`
    XCNamespace        string             `json:"xcNamespace,omitempty"`
    DiscoveredOrigins  []DiscoveredOrigin `json:"discoveredOrigins,omitempty"`  // NEW
}

type DiscoveredOrigin struct {
    Resource    ResourceRef `json:"resource"`
    Address     string      `json:"address,omitempty"`
    Port        uint32      `json:"port,omitempty"`
    AddressType string      `json:"addressType,omitempty"`  // "IP" or "FQDN"
    Status      string      `json:"status"`                 // "Resolved" or "Pending"
    Message     string      `json:"message,omitempty"`
}
```

### Example CRs

**Discover from LoadBalancer Service:**

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: my-pool
spec:
  port: 443
  originServers:
    - discover:
        resource:
          kind: Service
          name: my-nginx
          namespace: production
```

**Discover with NAT override:**

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: nat-pool
spec:
  port: 443
  originServers:
    - discover:
        resource:
          kind: Service
          name: my-service
          namespace: production
        addressOverride: "203.0.113.50"
```

**Mixed static and discovered origins:**

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: mixed-pool
spec:
  port: 443
  originServers:
    - publicIP:
        ip: "198.51.100.10"
    - discover:
        resource:
          kind: Ingress
          name: my-ingress
          namespace: web
    - discover:
        resource:
          kind: Gateway
          name: my-gateway
          namespace: gateway-system
        portOverride: 8443
```

**Discover from OpenShift Route:**

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: openshift-pool
spec:
  port: 443
  originServers:
    - discover:
        resource:
          kind: Route
          name: my-route
          namespace: my-project
```

## Discovery Resolution

### Service Type Resolution Priority

A Service can match multiple categories (e.g., a LoadBalancer Service with externalIPs). The resolver uses this priority order, taking the first match:

1. `spec.type: ExternalName` → ExternalName resolution
2. `spec.externalIPs` is non-empty → externalIPs resolution
3. `spec.type: LoadBalancer` → LoadBalancer resolution
4. `spec.type: NodePort` → NodePort resolution
5. `spec.type: ClusterIP` (with none of the above) → DiscoveryFailed (ClusterIP is not externally routable)

### Resolution Logic Per Resource Type

**Service (LoadBalancer):**
- Address: `.status.loadBalancer.ingress[0].ip` (→ IP) or `.status.loadBalancer.ingress[0].hostname` (→ FQDN)
- Port: `.spec.ports[0].port`
- Pending when: `.status.loadBalancer.ingress` is empty (cloud LB not yet provisioned)

**Service (NodePort):**
- Address: external IP of any Ready node (queried from Node objects)
- Port: `.spec.ports[0].nodePort`
- Pending when: no nodes have an external IP

**Service (ExternalName):**
- Address: `.spec.externalName` (→ FQDN)
- Port: `.spec.ports[0].port` if defined, otherwise uses OriginPool's top-level `port`
- Never pending (address is always available if Service exists)

**Service (externalIPs):**
- Address: `.spec.externalIPs[0]` (→ IP)
- Port: `.spec.ports[0].port`
- Never pending (externalIPs is user-specified, always present if set)

**Ingress:**
- Address: `.status.loadBalancer.ingress[0].ip` (→ IP) or `.status.loadBalancer.ingress[0].hostname` (→ FQDN)
- Port: 443 if any TLS rule is configured, otherwise 80
- Pending when: `.status.loadBalancer.ingress` is empty (ingress controller hasn't assigned an address)

**Gateway:**
- Address: `.status.addresses[0].value` (→ IP or FQDN based on `.status.addresses[0].type`)
- Port: `.spec.listeners[0].port`
- Pending when: `.status.addresses` is empty

**Route (OpenShift):**
- Address: `.status.ingress[0].host` (→ FQDN, this is the admitted route's hostname)
- Port: 443 if `.spec.tls` is configured, otherwise 80
- Pending when: `.status.ingress` is empty or no conditions show `Admitted=True`

### Override Behavior

- `addressOverride` set: use override address instead of discovered address. AddressType determined by whether override parses as an IP.
- `portOverride` set: use override port instead of discovered port.
- Both set: the referenced resource only triggers reconciliation on changes. Discovery status still tracks the resource state for observability.

### XC API Mapping

The resolved address maps to existing XC origin server types:
- IP address → `publicIP` with `ip` field
- FQDN → `publicName` with `dnsName` field

Port discovery feeds into the OriginPool's top-level `port` field only if there's a single origin server. For multi-origin pools, all origins must use the same port (XC API constraint — port is pool-level, not per-origin). If discovered ports differ from the pool's `port`, the controller logs a warning but uses the pool-level `port`.

## Controller Changes

### Reconcile Flow

The existing Get→Compare→Act flow gains a resolution step:

1. Fetch OriginPool CR *(existing)*
2. Check deletion / add finalizer *(existing)*
3. **Resolve all `discover` origin servers** *(new)*
   - For each `discover` entry, fetch the referenced K8s resource
   - Extract address and port using type-specific resolver
   - Apply overrides if specified
   - If ANY origin is pending: set `Ready=False, Reason=DiscoveryPending`, update `discoveredOrigins` status, requeue with 10s backoff, stop
4. Build XC create/replace payload with resolved addresses *(modified mapper)*
5. Get→Compare→Act against XC API *(existing)*
6. Update status including `discoveredOrigins` *(modified)*

No partial syncs: all origins must resolve before the controller syncs to XC.

### Dynamic Watches

The controller watches referenced resources so it re-reconciles when they change (e.g., a Service gets a new LoadBalancer IP).

**Implementation:** Controller-runtime field indexer + watch with mapping function:

- **Indexer:** Indexes OriginPool objects by their `discover` references (e.g., key `Service/production/my-nginx`). Set up at controller startup.
- **Watch mapper:** When a Service/Ingress/Gateway/Route changes, the mapper queries the index to find which OriginPools reference it, and enqueues those for reconciliation.
- **Cross-namespace:** OriginPools can reference resources in any namespace. Controller-runtime watches are cluster-scoped by Kind; the indexer handles the namespace→OriginPool mapping.

### Conditional Watch Registration

Not all clusters have Gateway API or OpenShift Route CRDs installed. The controller checks at startup:

- **Always registered:** Service, Ingress (core K8s types, always present)
- **Conditional:** Gateway (check for `gateway.networking.k8s.io` CRD), Route (check for `route.openshift.io` CRD)
- **Node watch:** Always registered alongside Service watches. Needed for NodePort resolution (node external IPs). Node changes trigger re-reconciliation of OriginPools that reference NodePort Services via the indexer.

If a user creates an OriginPool referencing a Gateway on a cluster without Gateway API, the controller sets `Ready=False, Reason=DiscoveryFailed, Message="Gateway CRD not installed"`.

### RBAC

New permissions needed:

```go
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch
```

## File Structure

### New Files

- `internal/controller/origin_resolver.go` — Pure resolution functions: given a K8s resource object, extract address+port. One function per resource type, plus a dispatcher.
- `internal/controller/origin_resolver_test.go` — Unit tests for each resolver function with all variants (IP, hostname, pending, overrides)
- `internal/controller/originpool_controller_discover_test.go` — Controller unit tests for discovery behavior (envtest + fake XC client)
- `internal/controller/originpool_integration_discover_test.go` — Integration tests for discovery flows (envtest + FakeXCServer)

### Modified Files

- `api/v1alpha1/originpool_types.go` — Add `Discover` field to OriginServer, add `DiscoveredOrigins` to OriginPoolStatus, add `DiscoveredOrigin` type
- `api/v1alpha1/shared_types.go` — Add `ResourceRef` type
- `internal/controller/originpool_controller.go` — Add resolution step, dynamic watch setup, conditional CRD checks, DiscoveryPending handling
- `internal/controller/originpool_mapper.go` — Accept resolved discover origins, map IP→publicIP / FQDN→publicName
- `api/v1alpha1/zz_generated.deepcopy.go` — Regenerated
- `config/crd/bases/xc.f5.com_originpools.yaml` — Regenerated with new fields
- `config/rbac/role.yaml` — Regenerated with new RBAC permissions

### Not Modified

- XC client (`internal/xcclient/`) — no changes needed
- Other CRD controllers — untouched
- `cmd/main.go` — no new controller to register

## Testing Strategy

### Resolver Unit Tests (`origin_resolver_test.go`)

Pure function tests, no cluster needed:

- **Service (LoadBalancer):** with IP → resolves IP; with hostname → resolves FQDN; no status → Pending
- **Service (NodePort):** with ready node → resolves node external IP + nodePort; no external IP nodes → Pending
- **Service (ExternalName):** resolves externalName as FQDN; with port → uses port; without port → no port
- **Service (externalIPs):** resolves first externalIP
- **Ingress:** with IP → resolves IP; with hostname → resolves FQDN; with TLS → port 443; without TLS → port 80; no status → Pending
- **Gateway:** with IP address → resolves IP; with hostname → resolves FQDN; no addresses → Pending
- **Route:** with admitted host → resolves FQDN; with TLS → port 443; without TLS → port 80; not admitted → Pending
- **Overrides:** addressOverride replaces address; portOverride replaces port; both override; IP vs FQDN detection on override
- **Unsupported Kind:** returns error

### Controller Unit Tests (`originpool_controller_discover_test.go`)

Envtest + fake XC client:

- OriginPool with `discover` origin resolves Service and syncs to XC as `publicIP`
- OriginPool with `discover` origin resolves Ingress hostname and syncs as `publicName`
- Discovery pending (no LB IP yet) → Ready=False, Reason=DiscoveryPending, no XC API call
- Referenced resource updates → OriginPool re-reconciles with new address
- Mixed static + discover origins: all resolve → syncs; one pending → blocks sync
- addressOverride: uses override address, ignores discovered
- portOverride: uses override port
- Unsupported Kind → Ready=False, Reason=DiscoveryFailed
- Missing referenced resource → Ready=False, Reason=DiscoveryFailed
- DiscoveredOrigins status reflects current resolution state

### Integration Tests (`originpool_integration_discover_test.go`)

Envtest + FakeXCServer:

- Create Service (LoadBalancer) → Create OriginPool referencing it → XC gets publicIP origin
- Service pending → OriginPool DiscoveryPending → Service gets IP → OriginPool becomes Ready → XC synced
- Service IP changes → OriginPool re-syncs → XC updated with new IP
- Delete OriginPool with discover origin → XC origin pool deleted (finalizer works)

### Contract Tests

Existing OriginPool contract tests already validate the XC API. Discovery maps to `publicIP`/`publicName` — same XC payload shape. A single additional contract test verifying a discovered-origin OriginPool creates successfully on the real API is sufficient.

## Constants

New reason constants in `api/v1alpha1/constants.go`:

```go
const (
    ReasonDiscoveryPending = "DiscoveryPending"
    ReasonDiscoveryFailed  = "DiscoveryFailed"
)
```

New address type constants:

```go
const (
    AddressTypeIP   = "IP"
    AddressTypeFQDN = "FQDN"
)
```

New discovery status constants:

```go
const (
    DiscoveryStatusResolved = "Resolved"
    DiscoveryStatusPending  = "Pending"
)
```
