# F5 Distributed Cloud Kubernetes Operator

A Kubernetes operator that manages [F5 Distributed Cloud](https://www.f5.com/cloud) resources declaratively. Define load balancers, origin pools, firewalls, and policies as Kubernetes custom resources and the operator syncs them to your F5 XC tenant.

Origin pools support automatic service discovery from Kubernetes Services, Ingress, Gateway API, and OpenShift Routes, so your F5 XC configuration stays in sync as endpoints change.

## Resources

| Kind | Short Name | Description |
|------|-----------|-------------|
| [HTTPLoadBalancer](docs/resources/httploadbalancer.md) | `hlb` | HTTP/HTTPS load balancers with WAF, bot defense, rate limiting |
| [TCPLoadBalancer](docs/resources/tcploadbalancer.md) | `tlb` | TCP load balancers with TLS termination or auto-cert |
| [OriginPool](docs/resources/originpool.md) | `op` | Backend server groups with optional auto-discovery from K8s Services, Ingress, Gateway, and Routes |
| [AppFirewall](docs/resources/appfirewall.md) | `afw` | Web Application Firewall policies |
| [ServicePolicy](docs/resources/servicepolicy.md) | `sp` | L7 request-level allow/deny policies |
| [HealthCheck](docs/resources/healthcheck.md) | `hc` | HTTP and TCP health checks for origin pools |
| [RateLimiter](docs/resources/ratelimiter.md) | `rl` | Rate limiting thresholds |
| [Certificate](docs/resources/certificate.md) | `cert` | TLS certificates from K8s Secrets, synced to XC |
| [APIDefinition](docs/resources/apidefinition.md) | `apidef` | API endpoint definitions with swagger specs |
| [UserIdentification](docs/resources/useridentification.md) | `uid` | User identification rules for security features |
| [MaliciousUserMitigation](docs/resources/malicioususermitigation.md) | `mum` | Threat-level response actions for malicious users |

## Quick Start

### Prerequisites

- Kubernetes 1.26+
- Helm 3
- An F5 XC tenant with an [API token or API certificate](https://docs.cloud.f5.com/docs/how-to/user-mgmt/credentials)

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

## Key Concepts

**`xcNamespace`** — Every resource requires this field to specify the F5 XC namespace. This is independent of the Kubernetes namespace. Resources that reference each other must share the same `xcNamespace`. Some types (AppFirewall, HealthCheck, ServicePolicy, Certificate) can use the `shared` XC namespace and be referenced from any application namespace.

**OneOf fields** — Many fields use a "pick one" pattern (e.g., `http` vs `https` vs `httpsAutoCert`). Set the one you want as a JSON object; leave the rest unset. See the [F5 XC API docs](https://docs.cloud.f5.com/docs-v2/api) for sub-field details.

**ObjectRef** — Resources reference each other by `name`. To reference a resource in a different K8s namespace, add `namespace`. Resources in the `shared` XC namespace (AppFirewall, HealthCheck, ServicePolicy, Certificate) can be referenced from any application namespace this way.

**Dependencies** — Resources can reference each other (e.g., HTTPLoadBalancer → OriginPool → HealthCheck). Apply all manifests at once — the operator reconciles with retries, so dependencies resolve automatically regardless of creation order.

**Status** — All resources report `Ready` and `Synced` conditions. `kubectl get <resource>` shows both at a glance.

**Deletion policy** — Annotate a CR with `f5xc.io/deletion-policy: orphan` to keep the XC resource when the CR is deleted.

## Documentation

- [Helm Values and Configuration](docs/overview.md)
- [Development Guide](docs/development.md)
- [Resource Guides](docs/resources/)
- [LLM/Agent Operating Guide](docs/llms-full.txt)
- [F5 XC API Reference](https://docs.cloud.f5.com/docs-v2/api)
