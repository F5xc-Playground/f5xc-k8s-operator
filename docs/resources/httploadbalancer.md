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
    addHSTS: true
    httpRedirect: true
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

> **Full field reference:** [HTTP Load Balancer API Documentation](https://docs.cloud.f5.com/docs-v2/api/views-http-loadbalancer)
>
> Fields marked as "object" below are JSON objects passed through to the XC API. The API docs describe all available sub-fields and OneOf options.

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
| `http` | Plain HTTP. Optional `port` (defaults to 80) and `dnsVolterraManaged` fields. |
| `https` | HTTPS with explicit TLS certificate configuration. |
| `httpsAutoCert` | HTTPS with automatic certificate from F5 XC. |

These are JSON objects passed through to the XC API.

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

Each group is mutually exclusive. Set at most one field per group.

| Group | Options |
|-------|---------|
| Bot defense | `disableBotDefense`, `botDefense` |
| API discovery | `disableAPIDiscovery`, `enableAPIDiscovery` |
| IP reputation | `disableIPReputation`, `enableIPReputation` |
| Rate limiting | `disableRateLimit`, `rateLimit` |
| Challenge | `noChallenge`, `jsChallenge`, `captchaChallenge`, `policyBasedChallenge` |
| LB algorithm | `roundRobin`, `leastActive`, `random`, `sourceIPStickiness`, `cookieStickiness`, `ringHash` |
| Service policies | `servicePoliciesFromNamespace`, `activeServicePolicies`, `noServicePolicies` |
| User ID | `userIDClientIP`, `userIdentification` (ObjectRef to UserIdentification CR) |
| API definition | `disableAPIDefinition`, `apiSpecification` (see below) |
| Malicious user detection | `disableMaliciousUserDetection`, `enableMaliciousUserDetection` |

### ObjectRef Fields

| Field | Type | Description |
|-------|------|-------------|
| `userIdentification` | ObjectRef | Reference to a UserIdentification CR |
| `apiSpecification` | APISpecificationConfig | API definition with validation settings (see below) |

### APISpecificationConfig

```yaml
apiSpecification:
  apiDefinition:
    name: my-api-def       # APIDefinition CR name
  validationDisabled: {}   # or validationAllMethods / validationCustomList
```

### PolicyBasedChallenge

When using `policyBasedChallenge`, you can reference a MaliciousUserMitigation CR:

```yaml
policyBasedChallenge:
  maliciousUserMitigation:
    name: my-mitigation    # MaliciousUserMitigation CR name
```

Note: `enableMaliciousUserDetection` must also be set at the top level to activate detection.
