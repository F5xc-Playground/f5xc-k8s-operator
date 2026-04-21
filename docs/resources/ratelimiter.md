# RateLimiter

A RateLimiter defines a rate limiting threshold that can be referenced by HTTPLoadBalancers. It controls how many requests per time unit are allowed before traffic is throttled.

**Short name:** `rl`

## F5 XC Documentation

- [Rate Limiting (How-To)](https://docs.cloud.f5.com/docs/how-to/advanced-security/user-rate-limit)
- [Rate Limiter API Reference](https://docs.cloud.f5.com/docs-v2/api/rate-limiter)

## Examples

### Basic rate limiter

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: RateLimiter
metadata:
  name: standard-limit
spec:
  xcNamespace: my-namespace
  threshold: 100
  unit: "MINUTE"
```

### Strict rate limiter with burst

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: RateLimiter
metadata:
  name: strict-limit
spec:
  xcNamespace: my-namespace
  threshold: 10
  unit: "SECOND"
  burstMultiplier: 3
```

### Reference from an HTTPLoadBalancer

Rate limiters are referenced from HTTPLoadBalancers using the `rateLimit` OneOf field. The exact structure depends on your XC configuration -- see the [HTTP Load Balancer API docs](https://docs.cloud.f5.com/docs-v2/api/views-http-loadbalancer) for details.

## Spec Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `threshold` | int | Yes | Maximum number of requests per time unit |
| `unit` | string | Yes | Time unit: `SECOND`, `MINUTE`, or `HOUR` |
| `burstMultiplier` | int | No | Multiplier for burst allowance above the threshold |
