# OriginPool

An OriginPool defines a set of backend servers (origins) that load balancers route traffic to. Origins can be static IPs, DNS names, Kubernetes Services, or dynamically discovered from cluster resources.

**Short name:** `op`

## F5 XC Documentation

- [Create Origin Pools (How-To)](https://docs.cloud.f5.com/docs/how-to/app-networking/origin-pools)
- [Origin Pool API Reference](https://docs.cloud.f5.com/docs-v2/api/views-origin-pool)

## Examples

### Static IP origin

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: static-pool
spec:
  xcNamespace: my-namespace
  port: 443
  originServers:
    - publicIP:
        ip: "203.0.113.10"
    - publicIP:
        ip: "203.0.113.11"
  loadBalancerAlgorithm: "ROUND_ROBIN"
```

### DNS name origin

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: dns-pool
spec:
  xcNamespace: my-namespace
  port: 443
  originServers:
    - publicName:
        dnsName: "backend.example.com"
```

### Kubernetes Service origin (via site)

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: k8s-pool
spec:
  xcNamespace: my-namespace
  port: 8080
  originServers:
    - k8sService:
        serviceName: my-service
        serviceNamespace: default
        site:
          name: my-xc-site
```

### Auto-discovery from a Kubernetes Service

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: discovered-pool
spec:
  xcNamespace: my-namespace
  port: 443
  originServers:
    - discover:
        resource:
          kind: Service
          name: my-nginx
          namespace: default
```

The operator watches the referenced resource and resolves its address automatically. Discovered addresses appear in `.status.discoveredOrigins`. Supported resource kinds: `Service`, `Ingress`, `Gateway`, `Route` (OpenShift).

You can override the discovered address or port:

```yaml
    - discover:
        resource:
          kind: Service
          name: my-nginx
          namespace: default
        addressOverride: "10.0.0.5"
        portOverride: 8443
```

### With health check and TLS

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: tls-pool
spec:
  xcNamespace: my-namespace
  port: 443
  originServers:
    - publicIP:
        ip: "203.0.113.10"
  healthChecks:
    - name: my-healthcheck
  useTLS:
    skip_server_verification: {}
```

## Spec Reference

> **Full field reference:** [Origin Pool API Documentation](https://docs.cloud.f5.com/docs-v2/api/views-origin-pool)
>
> Fields marked as "object" below are JSON objects passed through to the XC API. The API docs describe all available sub-fields.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `port` | int | Yes | Port for all origin servers |
| `originServers` | list | Yes | List of origin server definitions (see below) |
| `loadBalancerAlgorithm` | string | No | Algorithm: `ROUND_ROBIN`, `LEAST_REQUEST`, `RANDOM`, `RING_HASH` |
| `healthChecks` | list of ObjectRef | No | References to HealthCheck CRs |
| `useTLS` | object | No | TLS config (OneOf with `noTLS`) |
| `noTLS` | object | No | Explicit no-TLS (OneOf with `useTLS`) |

### Origin Server Types

Each entry in `originServers` must specify exactly one of:

| Field | Description |
|-------|-------------|
| `publicIP.ip` | Public IP address |
| `publicName.dnsName` | Public DNS name |
| `privateIP.ip` | Private IP (with `site` or `virtualSite` and network choice) |
| `privateName.dnsName` | Private DNS name (with `site` or `virtualSite` and network choice) |
| `k8sService.serviceName` | Kubernetes Service (with optional `serviceNamespace`, `site`/`virtualSite`, and network choice) |
| `consulService.serviceName` | Consul service (with `site` or `virtualSite` and network choice) |
| `discover.resource` | Auto-discover from a K8s resource (`kind`, `name`, `namespace`) |

### Site Location

Private, K8S, and Consul origin servers require a site or virtual site reference. Use `site` for a specific CE site, or `virtualSite` for a virtual site selector:

```yaml
- privateIP:
    ip: "10.0.0.1"
    site:
      name: my-ce-site
    outsideNetwork: {}
```

```yaml
- privateIP:
    ip: "10.0.0.1"
    virtualSite:
      name: my-vsite
    insideNetwork: {}
```

### Network Choice

For private/K8S/Consul origins, you can specify which network interface to use:

| Field | Description |
|-------|-------------|
| `insideNetwork` | `{}` to use the inside network interface |
| `outsideNetwork` | `{}` to use the outside network interface |

## Status

| Field | Description |
|-------|-------------|
| `conditions` | `Ready` and `Synced` conditions |
| `xcUID` | F5 XC resource identifier |
| `discoveredOrigins` | Resolved addresses from `discover` entries (address, port, addressType, status) |
