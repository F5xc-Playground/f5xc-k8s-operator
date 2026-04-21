# HealthCheck

A HealthCheck defines how F5 XC monitors the health of origin servers. Health checks can be referenced by OriginPool resources to automatically remove unhealthy backends from rotation.

**Short name:** `hc`

## F5 XC Documentation

- [Health Checks (How-To)](https://docs.cloud.f5.com/docs/how-to/app-networking/origin-pools)
- [Health Check API Reference](https://docs.cloud.f5.com/docs-v2/api/healthcheck)

## Examples

### HTTP health check

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HealthCheck
metadata:
  name: http-check
spec:
  xcNamespace: my-namespace
  httpHealthCheck:
    path: "/healthz"
```

### HTTP health check with thresholds

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HealthCheck
metadata:
  name: strict-check
spec:
  xcNamespace: my-namespace
  httpHealthCheck:
    path: "/health"
    expectedStatusCodes:
      - "200"
      - "204"
  healthyThreshold: 3
  unhealthyThreshold: 2
  interval: 10
  timeout: 5
```

### TCP health check

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HealthCheck
metadata:
  name: tcp-check
spec:
  xcNamespace: my-namespace
  tcpHealthCheck: {}
```

### Attach to an OriginPool

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: monitored-pool
spec:
  xcNamespace: my-namespace
  port: 443
  originServers:
    - publicIP:
        ip: "203.0.113.10"
  healthChecks:
    - name: http-check
```

## Spec Reference

> **Full field reference:** [Health Check API Documentation](https://docs.cloud.f5.com/docs-v2/api/healthcheck)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `httpHealthCheck` | object | No | HTTP health check config (see below) |
| `tcpHealthCheck` | object | No | TCP health check config (see below) |
| `healthyThreshold` | int | No | Consecutive successes before marking healthy |
| `unhealthyThreshold` | int | No | Consecutive failures before marking unhealthy |
| `interval` | int | No | Check interval in seconds |
| `timeout` | int | No | Check timeout in seconds |
| `jitterPercent` | int | No | Random jitter as a percentage (0-100) |

### HTTP Health Check

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | HTTP path to request (e.g., `/healthz`) |
| `useHTTP2` | bool | Use HTTP/2 for the health check |
| `expectedStatusCodes` | list of string | HTTP status codes that indicate healthy (e.g., `["200", "204"]`) |

### TCP Health Check

| Field | Type | Description |
|-------|------|-------------|
| `send` | string | Payload to send |
| `receive` | string | Expected response |
