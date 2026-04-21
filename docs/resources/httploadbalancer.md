# HTTPLoadBalancer

An HTTPLoadBalancer exposes one or more domains over HTTP or HTTPS, routing traffic to OriginPool backends. It supports WAF integration, bot defense, rate limiting, challenge pages, and multiple advertisement strategies.

**Short name:** `hlb`

## F5 XC Documentation

- [Create HTTP Load Balancer (How-To)](https://docs.cloud.f5.com/docs/how-to/app-networking/http-load-balancer)
- [HTTP Load Balancer API Reference](https://docs.cloud.f5.com/docs-v2/api/views-http-loadbalancer)

## Examples

### Basic HTTP load balancer

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: my-lb
spec:
  xcNamespace: my-namespace
  domains:
    - "app.example.com"
  http:
    port: 80
  defaultRoutePools:
    - pool:
        name: my-origin-pool
      weight: 1
  advertiseOnPublicDefaultVIP: {}
```

### HTTPS with automatic certificate

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: https-lb
spec:
  xcNamespace: my-namespace
  domains:
    - "secure.example.com"
  httpsAutoCert:
    add_hsts: true
    http_redirect: true
  defaultRoutePools:
    - pool:
        name: my-origin-pool
      weight: 1
  advertiseOnPublicDefaultVIP: {}
```

### With WAF and service policies

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: protected-lb
spec:
  xcNamespace: my-namespace
  domains:
    - "app.example.com"
  http:
    port: 80
  defaultRoutePools:
    - pool:
        name: my-origin-pool
      weight: 1
  appFirewall:
    name: my-waf
  servicePoliciesFromNamespace: {}
  advertiseOnPublicDefaultVIP: {}
```

## Spec Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `domains` | list of string | Yes | Domain names to serve |
| `defaultRoutePools` | list of RoutePool | Yes | Default backend pools |
| `routes` | list of object | No | Advanced routing rules (pass-through to XC API) |

### RoutePool

```yaml
defaultRoutePools:
  - pool:
      name: my-pool        # OriginPool CR name
      namespace: other-ns   # optional
    weight: 1               # optional, for weighted routing
    priority: 1             # optional
```

### TLS Mode (choose one)

| Field | Description |
|-------|-------------|
| `http` | Plain HTTP. Requires `port` inside the object. |
| `https` | HTTPS with explicit TLS certificate configuration. |
| `httpsAutoCert` | HTTPS with automatic certificate from F5 XC. |

These are JSON objects passed through to the XC API. See the [API reference](https://docs.cloud.f5.com/docs-v2/api/views-http-loadbalancer) for available sub-fields.

### WAF (choose one)

| Field | Description |
|-------|-------------|
| `appFirewall` | ObjectRef to an AppFirewall CR |
| `disableWAF` | `{}` to explicitly disable |

### Advertisement (choose one)

| Field | Description |
|-------|-------------|
| `advertiseOnPublicDefaultVIP` | `{}` to advertise on the default public VIP |
| `advertiseOnPublic` | Custom public advertisement config |
| `advertiseCustom` | Custom advertisement config |
| `doNotAdvertise` | `{}` to not advertise |

### Additional OneOf Groups

Each group is mutually exclusive. Set at most one field per group. All accept JSON objects; refer to the [XC API docs](https://docs.cloud.f5.com/docs-v2/api/views-http-loadbalancer) for the structure.

| Group | Options |
|-------|---------|
| Bot defense | `disableBotDefense`, `botDefense` |
| API discovery | `disableAPIDiscovery`, `enableAPIDiscovery` |
| IP reputation | `disableIPReputation`, `enableIPReputation` |
| Rate limiting | `disableRateLimit`, `rateLimit` |
| Challenge | `noChallenge`, `jsChallenge`, `captchaChallenge`, `policyBasedChallenge` |
| LB algorithm | `roundRobin`, `leastActive`, `random`, `sourceIPStickiness`, `cookieStickiness`, `ringHash` |
| Service policies | `servicePoliciesFromNamespace`, `activeServicePolicies`, `noServicePolicies` |
| User ID | `userIDClientIP` |
