# TCPLoadBalancer

A TCPLoadBalancer exposes a TCP service on one or more domains, routing traffic to OriginPool backends. It supports TLS termination, TLS passthrough, and multiple advertisement strategies.

**Short name:** `tlb`

## F5 XC Documentation

- [Create TCP Load Balancer (How-To)](https://docs.cloud.f5.com/docs/how-to/app-networking/tcp-load-balancer)
- [TCP Load Balancer API Reference](https://docs.cloud.f5.com/docs-v2/api/views-tcp-loadbalancer)

## Examples

### Basic TCP load balancer

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: TCPLoadBalancer
metadata:
  name: my-tcp-lb
spec:
  xcNamespace: my-namespace
  domains:
    - "tcp.example.com"
  listenPort: 443
  originPools:
    - pool:
        name: my-origin-pool
      weight: 1
```

### With TLS passthrough

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: TCPLoadBalancer
metadata:
  name: tls-passthrough-lb
spec:
  xcNamespace: my-namespace
  domains:
    - "tcp.example.com"
  listenPort: 443
  originPools:
    - pool:
        name: my-origin-pool
      weight: 1
  tlsPassthrough: {}
  advertiseOnPublicDefaultVIP: {}
```

## Spec Reference

> **Full field reference:** [TCP Load Balancer API Documentation](https://docs.cloud.f5.com/docs-v2/api/views-tcp-loadbalancer)
>
> Fields marked as "object" below are JSON objects passed through to the XC API. The API docs describe all available sub-fields and OneOf options.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `domains` | list of string | Yes | Domain names to serve |
| `listenPort` | int | Yes | TCP listening port |
| `originPools` | list of RoutePool | Yes | Backend pools |

### TLS Mode (choose one)

| Field | Description |
|-------|-------------|
| `noTLS` | `{}` for plain TCP |
| `tlsParameters` | TLS termination with certificate config |
| `tlsPassthrough` | `{}` to pass TLS through to the backend |

### Advertisement (choose one)

| Field | Description |
|-------|-------------|
| `advertiseOnPublicDefaultVIP` | `{}` to advertise on the default public VIP |
| `advertiseOnPublic` | Custom public advertisement config |
| `advertiseCustom` | Custom advertisement config |
| `doNotAdvertise` | `{}` to not advertise |

All OneOf fields accept JSON objects passed through to the XC API.
