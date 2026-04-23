# Observability

The operator exposes Prometheus metrics on port `8080` (configurable via `--metrics-bind-address`) and health probes on port `8081`.

## Metrics

### Operator Metrics

Custom metrics for the F5 XC API client:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `f5xc_api_requests_total` | Counter | `endpoint`, `method`, `status_code` | Total F5 XC API requests |
| `f5xc_api_request_duration_seconds` | Histogram | `endpoint`, `method` | F5 XC API request latency |
| `f5xc_api_rate_limit_hits_total` | Counter | `endpoint` | Rate limit responses from F5 XC |
| `f5xc_api_retries_total` | Counter | `endpoint`, `reason` | Retried API requests |
| `f5xc_api_updates_skipped_total` | Counter | `endpoint` | Updates skipped because the spec hadn't changed |

### Controller Runtime Metrics

The operator also exposes standard [controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference), including:

- `controller_runtime_reconcile_total` — reconciliation count by controller and result
- `controller_runtime_reconcile_time_seconds` — reconciliation duration
- `workqueue_depth` — current work queue depth
- `workqueue_adds_total` — items added to work queue

These are available at the same `/metrics` endpoint.

## Scraping with Prometheus

The operator pod exposes metrics on port `8080` at `/metrics`. There is no Service or ServiceMonitor included in the Helm chart, so you need to configure scraping for your environment.

### Prometheus ServiceMonitor (kube-prometheus-stack)

Create a Service and ServiceMonitor to scrape the operator:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: f5xc-operator-metrics
  namespace: f5xc-system
  labels:
    app: f5xc-operator
spec:
  selector:
    app: f5xc-k8s-operator
  ports:
    - name: metrics
      port: 8080
      targetPort: metrics
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: f5xc-operator
  namespace: f5xc-system
spec:
  selector:
    matchLabels:
      app: f5xc-operator
  endpoints:
    - port: metrics
      interval: 30s
```

### Prometheus Pod Annotations

If your Prometheus is configured to scrape annotated pods, add annotations to the operator deployment:

```yaml
# In your Helm values or deployment patch:
spec:
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
```

## Health Probes

| Endpoint | Port | Description |
|----------|------|-------------|
| `/healthz` | 8081 | Liveness — operator process is running |
| `/readyz` | 8081 | Readiness — operator is ready to reconcile |

## Example Alerts

```yaml
groups:
  - name: f5xc-operator
    rules:
      - alert: F5XCAPIErrorRate
        expr: |
          rate(f5xc_api_requests_total{status_code=~"4..|5.."}[5m])
          / rate(f5xc_api_requests_total[5m]) > 0.1
        for: 5m
        annotations:
          summary: "F5 XC API error rate above 10%"

      - alert: F5XCAPIRateLimited
        expr: rate(f5xc_api_rate_limit_hits_total[5m]) > 0
        for: 5m
        annotations:
          summary: "Operator is being rate limited by F5 XC API"

      - alert: F5XCReconcileErrors
        expr: rate(controller_runtime_reconcile_total{result="error"}[5m]) > 0
        for: 10m
        annotations:
          summary: "Controller reconciliation errors sustained for 10m"
```
