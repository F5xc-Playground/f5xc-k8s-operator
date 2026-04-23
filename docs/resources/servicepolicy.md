# ServicePolicy

A ServicePolicy defines L7 request-level access control rules. Policies can allow or deny traffic based on IP prefixes, headers, paths, and other request attributes. They are applied to HTTPLoadBalancers via the `servicePoliciesFromNamespace` or `activeServicePolicies` field.

**Short name:** `sp`

## F5 XC Documentation

- [Service Policy (How-To)](https://docs.cloud.f5.com/docs/how-to/app-security/service-policy)
- [Service Policy API Reference](https://docs.cloud.f5.com/docs-v2/api/service-policy)

## Examples

### Allow all requests

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: ServicePolicy
metadata:
  name: allow-all
spec:
  xcNamespace: my-namespace
  allowAllRequests: {}
  anyServer: {}
```

### Deny all requests

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: ServicePolicy
metadata:
  name: deny-all
spec:
  xcNamespace: my-namespace
  denyAllRequests: {}
  anyServer: {}
```

### Allow list with IP rules

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: ServicePolicy
metadata:
  name: internal-only
spec:
  xcNamespace: my-namespace

  allowList:
    prefixes:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
    defaultActionDeny: {}
  anyServer: {}
```

### Deny list

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: ServicePolicy
metadata:
  name: block-bad-actors
spec:
  xcNamespace: my-namespace

  denyList:
    prefixes:
      - "198.51.100.0/24"
    defaultActionAllow: {}
  anyServer: {}
```

## Spec Reference

> **Full field reference:** [Service Policy API Documentation](https://docs.cloud.f5.com/docs-v2/api/service-policy)
>
> Fields marked as "object" below are JSON objects passed through to the XC API. The API docs describe all available sub-fields, rule schemas, and match conditions.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |

### Rule Choice (choose one)

| Field | Description |
|-------|-------------|
| `allowAllRequests` | `{}` to allow everything |
| `denyAllRequests` | `{}` to deny everything |
| `allowList` | Allow rules with a default deny/allow action |
| `denyList` | Deny rules with a default allow/deny action |
| `ruleList` | Custom rules with explicit actions per rule |

### Server Choice (choose one)

Controls which servers this policy applies to.

| Field | Description |
|-------|-------------|
| `anyServer` | `{}` to apply to all servers |
| `serverName` | Match a specific server name (string) |
| `serverNameMatcher` | Match server names by pattern |
| `serverSelector` | Match servers by label selector |

All OneOf fields accept JSON objects passed through to the XC API.
