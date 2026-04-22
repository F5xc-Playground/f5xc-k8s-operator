# UserIdentification

A UserIdentification defines how to identify unique users for security features like malicious user detection and rate limiting. Rules are evaluated sequentially (max 4 rules). It is referenced from an HTTPLoadBalancer via the `userIdentification` field.

**Short name:** `uid`

## F5 XC Documentation

- [User Identification (How-To)](https://docs.cloud.f5.com/docs/how-to/app-security/user-identification)
- [User Identification API Reference](https://docs.cloud.f5.com/docs-v2/api/user-identification)

## Examples

### Client IP only

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: UserIdentification
metadata:
  name: by-ip
spec:
  xcNamespace: my-namespace
  rules:
    - clientIP: {}
```

### Cookie-based identification

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: UserIdentification
metadata:
  name: by-cookie
spec:
  xcNamespace: my-namespace
  rules:
    - cookieName: SessionID
```

### Multiple rules (fallback chain)

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: UserIdentification
metadata:
  name: multi-rule
spec:
  xcNamespace: my-namespace
  rules:
    - jwtClaimName: sub
    - httpHeaderName: X-User-ID
    - clientIP: {}
```

### TLS fingerprint with IP

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: UserIdentification
metadata:
  name: tls-fp
spec:
  xcNamespace: my-namespace
  rules:
    - ipAndTLSFingerprint: {}
```

### Reference from an HTTPLoadBalancer

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: my-app
spec:
  xcNamespace: my-namespace
  # ...
  userIdentification:
    name: by-ip
```

## Spec Reference

> **Full field reference:** [User Identification API Documentation](https://docs.cloud.f5.com/docs-v2/api/user-identification)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `rules` | []UserIdentificationRule | Yes | 1-4 identification rules, evaluated sequentially |

### UserIdentificationRule (choose one per rule)

| Field | Type | Description |
|-------|------|-------------|
| `none` | `{}` | No identification |
| `clientIP` | `{}` | Identify by client IP address |
| `clientASN` | `{}` | Identify by autonomous system number |
| `clientCity` | `{}` | Identify by client city |
| `clientCountry` | `{}` | Identify by client country |
| `clientRegion` | `{}` | Identify by client region |
| `cookieName` | string | Identify by cookie value |
| `httpHeaderName` | string | Identify by HTTP header value |
| `ipAndHTTPHeaderName` | string | Identify by IP + HTTP header combination |
| `ipAndTLSFingerprint` | `{}` | Identify by IP + TLS fingerprint |
| `ipAndJA4TLSFingerprint` | `{}` | Identify by IP + JA4 TLS fingerprint |
| `tlsFingerprint` | `{}` | Identify by TLS fingerprint alone |
| `ja4TLSFingerprint` | `{}` | Identify by JA4 TLS fingerprint alone |
| `jwtClaimName` | string | Identify by JWT claim value |
| `queryParamKey` | string | Identify by query parameter value |
