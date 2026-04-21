# F5 Distributed Cloud Kubernetes Operator

A Kubernetes operator that manages F5 Distributed Cloud resources declaratively. Define load balancers, origin pools, firewalls, and policies as Kubernetes custom resources -- the operator syncs them to your F5 XC tenant.

## Resources

| Kind | Short Name | Description |
|------|-----------|-------------|
| OriginPool | `op` | Backend server groups with optional auto-discovery from K8s Services, Ingress, Gateway, and Routes |
| HTTPLoadBalancer | `hlb` | HTTP/HTTPS load balancers with WAF, bot defense, rate limiting |
| TCPLoadBalancer | `tlb` | TCP load balancers with TLS termination or passthrough |
| AppFirewall | `afw` | Web Application Firewall policies |
| HealthCheck | `hc` | HTTP and TCP health checks for origin pools |
| ServicePolicy | `sp` | L7 request-level allow/deny policies |
| RateLimiter | `rl` | Rate limiting thresholds |

## Quick Start

### Prerequisites

- Kubernetes 1.26+
- Helm 3
- An F5 XC tenant with an [API token](https://docs.cloud.f5.com/docs/how-to/user-mgmt/credentials)

### Install

```bash
helm install f5xc-operator ./charts/f5xc-k8s-operator \
  --set tenantURL=https://YOUR-TENANT.console.ves.volterra.io \
  --set credentials.apiToken=YOUR_API_TOKEN \
  --namespace f5xc-system --create-namespace
```

### Deploy a Resource

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: my-pool
spec:
  xcNamespace: my-xc-namespace
  originServers:
    - publicIP:
        ip: "203.0.113.10"
  port: 443
```

```bash
kubectl apply -f my-pool.yaml
kubectl get op my-pool
# NAME      READY   SYNCED   AGE
# my-pool   True    True     10s
```

## Documentation

- **[Overview](docs/overview.md)** -- Concepts, Helm values, and architecture
- **Resource Guides:**
  [OriginPool](docs/resources/originpool.md) |
  [HTTPLoadBalancer](docs/resources/httploadbalancer.md) |
  [TCPLoadBalancer](docs/resources/tcploadbalancer.md) |
  [AppFirewall](docs/resources/appfirewall.md) |
  [HealthCheck](docs/resources/healthcheck.md) |
  [ServicePolicy](docs/resources/servicepolicy.md) |
  [RateLimiter](docs/resources/ratelimiter.md)

## Key Concepts

**`xcNamespace`** -- Every resource requires this field to specify the F5 XC namespace. It is independent of the Kubernetes namespace. Resources that reference each other must share the same `xcNamespace`.

**OneOf fields** -- Many fields use a "pick one" pattern (e.g., `http` vs `https` vs `httpsAutoCert`). Set the one you want as a JSON object; leave the rest unset. See the [F5 XC API docs](https://docs.cloud.f5.com/docs-v2/api) for sub-field details.

**Status** -- All resources report `Ready` and `Synced` conditions. `kubectl get <resource>` shows both at a glance.

**Deletion policy** -- Annotate a CR with `f5xc.io/deletion-policy: orphan` to keep the XC resource when the CR is deleted.

## Development

```bash
make test              # Unit + integration tests (requires envtest)
make test-contract     # Contract tests against a live XC tenant
make manifests         # Regenerate CRDs
make generate          # Regenerate deepcopy methods
```
