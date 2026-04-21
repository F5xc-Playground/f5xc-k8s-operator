# F5 Distributed Cloud Kubernetes Operator

A Kubernetes operator that manages [F5 Distributed Cloud](https://www.f5.com/cloud) resources declaratively using Custom Resources. Define your load balancers, origin pools, firewalls, and policies as Kubernetes objects and the operator syncs them to your F5 XC tenant.

## Supported Resources

| Kind | Short Name | Description |
|------|-----------|-------------|
| [OriginPool](resources/originpool.md) | `op` | Backend server groups with optional auto-discovery from K8s Services, Ingress, Gateway, and Routes |
| [HTTPLoadBalancer](resources/httploadbalancer.md) | `hlb` | HTTP/HTTPS load balancers with WAF, bot defense, rate limiting, and more |
| [TCPLoadBalancer](resources/tcploadbalancer.md) | `tlb` | TCP load balancers with TLS termination or passthrough |
| [AppFirewall](resources/appfirewall.md) | `afw` | Web Application Firewall policies |
| [HealthCheck](resources/healthcheck.md) | `hc` | HTTP and TCP health checks for origin pools |
| [ServicePolicy](resources/servicepolicy.md) | `sp` | L7 request-level allow/deny policies |
| [RateLimiter](resources/ratelimiter.md) | `rl` | Rate limiting thresholds |

## Quick Start

### Prerequisites

- A Kubernetes cluster (1.26+)
- Helm 3
- An F5 Distributed Cloud tenant with an [API token](https://docs.cloud.f5.com/docs/how-to/user-mgmt/credentials)

### Install

```bash
helm install f5xc-operator ./charts/f5xc-k8s-operator \
  --set tenantURL=https://YOUR-TENANT.console.ves.volterra.io \
  --set credentials.apiToken=YOUR_API_TOKEN \
  --namespace f5xc-system --create-namespace
```

### Create a Resource

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
kubectl get originpool my-pool
# NAME      READY   SYNCED   AGE
# my-pool   True    True     10s
```

## Key Concepts

### XC Namespace

Every resource requires an `xcNamespace` field that specifies the F5 XC namespace to create the resource in. This is independent of the Kubernetes namespace -- a single cluster can manage resources across multiple XC namespaces.

Resources that reference each other (e.g., an HTTPLoadBalancer referencing an OriginPool) must share the same `xcNamespace`. The operator validates this and rejects cross-namespace references.

### OneOf Fields

Many F5 XC resources use a "OneOf" pattern where you pick exactly one option from a group. For example, an HTTPLoadBalancer's TLS mode is one of `http`, `https`, or `httpsAutoCert`. Set the one you want as an object; leave the rest unset.

These fields accept arbitrary JSON objects, which are passed through to the XC API. Refer to the [F5 XC API documentation](https://docs.cloud.f5.com/docs-v2/api) for the structure of each option.

### Status Conditions

All resources report two conditions:

- **Ready** -- the resource exists in F5 XC and is operational
- **Synced** -- the resource's current spec matches what's in F5 XC

Common `Synced` reasons: `CreateSucceeded`, `UpdateSucceeded`, `UpToDate`, `CreateFailed`, `UpdateFailed`, `AuthFailure`, `RateLimited`.

### Deletion Policy

By default, deleting a CR also deletes the corresponding F5 XC resource. To keep the XC resource when the CR is deleted (orphan it), annotate the CR:

```yaml
metadata:
  annotations:
    f5xc.io/deletion-policy: orphan
```

### Resource References

Some resources reference others using an `ObjectRef`:

```yaml
appFirewall:
  name: my-waf           # CR name in the same K8s namespace
  namespace: shared-ns    # optional: different K8s namespace
  tenant: my-tenant       # optional: XC tenant
```

## Helm Values

| Value | Default | Description |
|-------|---------|-------------|
| `tenantURL` | `""` | F5 XC tenant URL (required) |
| `credentials.secretName` | `xc-credentials` | Name of the K8s Secret containing credentials |
| `credentials.apiToken` | `""` | API token (stored in Secret at install) |
| `image.repository` | `f5xc-k8s-operator` | Container image repository |
| `image.tag` | `latest` | Container image tag |
| `replicaCount` | `1` | Number of operator replicas |
| `leaderElection` | `false` | Enable leader election for HA |
| `resources.requests.cpu` | `100m` | CPU request |
| `resources.requests.memory` | `128Mi` | Memory request |
| `resources.limits.cpu` | `500m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |

## F5 XC Documentation

- [F5 Distributed Cloud Concepts](https://docs.cloud.f5.com/docs/ves-concepts/load-balancing-and-proxy)
- [API Reference](https://docs.cloud.f5.com/docs-v2/api)
- [Generate API Credentials](https://docs.cloud.f5.com/docs/how-to/user-mgmt/credentials)
