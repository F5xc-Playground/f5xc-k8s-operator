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
  algo: "FIRST_MATCH"
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
  algo: "FIRST_MATCH"
  allowList:
    rules:
      - metadata:
          name: allow-internal
        spec:
          ip_prefix_list:
            prefix:
              - "10.0.0.0/8"
              - "192.168.0.0/16"
    default_action_deny: {}
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
  algo: "FIRST_MATCH"
  denyList:
    rules:
      - metadata:
          name: block-ips
        spec:
          ip_prefix_list:
            prefix:
              - "198.51.100.0/24"
    default_action_allow: {}
  anyServer: {}
```

## Spec Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `algo` | string | No | Rule matching algorithm (e.g., `FIRST_MATCH`) |

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
| `serverName` | Match a specific server name |
| `serverNameMatcher` | Match server names by pattern |
| `serverSelector` | Match servers by label selector |

All OneOf fields accept JSON objects passed through to the XC API. See the [API reference](https://docs.cloud.f5.com/docs-v2/api/service-policy) for the full rule schema.
