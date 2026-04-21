# Configuration

## Helm Values

| Value | Default | Description |
|-------|---------|-------------|
| `tenantURL` | `""` | F5 XC tenant URL (required) |
| `credentials.secretName` | `xc-credentials` | Name of the K8s Secret containing credentials |
| `credentials.apiToken` | `""` | API token (stored in a Secret at install time) |
| `image.repository` | `f5xc-k8s-operator` | Container image repository |
| `image.tag` | `latest` | Container image tag |
| `replicaCount` | `1` | Number of operator replicas |
| `leaderElection` | `false` | Enable leader election for HA deployments |
| `resources.requests.cpu` | `100m` | CPU request |
| `resources.requests.memory` | `128Mi` | Memory request |
| `resources.limits.cpu` | `500m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |

## XC Namespace

Every resource requires an `xcNamespace` field that specifies the F5 XC namespace to create the resource in. This is independent of the Kubernetes namespace — a single cluster can manage resources across multiple XC namespaces.

Resources that reference each other (e.g., an HTTPLoadBalancer referencing an OriginPool) must share the same `xcNamespace`. The operator validates this at reconciliation time and rejects cross-namespace references.

## OneOf Fields

Many F5 XC resources use a "OneOf" pattern where you pick exactly one option from a group. For example, an HTTPLoadBalancer's TLS mode is one of `http`, `https`, or `httpsAutoCert`. Set the one you want as an object; leave the rest unset.

These fields accept arbitrary JSON objects, which are passed through to the XC API as-is. Refer to the [F5 XC API documentation](https://docs.cloud.f5.com/docs-v2/api) for the structure of each option.

## Status Conditions

All resources report two conditions:

- **Ready** — the resource exists in F5 XC and is operational
- **Synced** — the resource's current spec matches what is in F5 XC

Common `Synced` reasons: `CreateSucceeded`, `UpdateSucceeded`, `UpToDate`, `CreateFailed`, `UpdateFailed`, `AuthFailure`, `RateLimited`.

## Deletion Policy

By default, deleting a CR also deletes the corresponding F5 XC resource. To keep the XC resource when the CR is deleted (orphan it), annotate the CR:

```yaml
metadata:
  annotations:
    f5xc.io/deletion-policy: orphan
```

## Resource References

Some resources reference others using an `ObjectRef`:

```yaml
appFirewall:
  name: my-waf           # CR name in the same K8s namespace
  namespace: shared-ns    # optional: different K8s namespace
  tenant: my-tenant       # optional: XC tenant
```

## F5 XC Documentation

- [F5 Distributed Cloud Concepts](https://docs.cloud.f5.com/docs/ves-concepts/load-balancing-and-proxy)
- [API Reference](https://docs.cloud.f5.com/docs-v2/api)
- [Generate API Credentials](https://docs.cloud.f5.com/docs/how-to/user-mgmt/credentials)
