# AppFirewall

An AppFirewall defines a Web Application Firewall (WAF) policy that can be attached to an HTTPLoadBalancer. It controls detection settings, enforcement mode (monitoring vs. blocking), and bot protection.

**Short name:** `afw`

## F5 XC Documentation

- [App Firewall (How-To)](https://docs.cloud.f5.com/docs/how-to/app-security/web-app-firewall)
- [App Firewall API Reference](https://docs.cloud.f5.com/docs-v2/api/app-firewall)

## Examples

### Blocking mode (default detection)

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: AppFirewall
metadata:
  name: my-waf
spec:
  xcNamespace: my-namespace
  blocking: {}
```

### Monitoring mode

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: AppFirewall
metadata:
  name: monitor-waf
spec:
  xcNamespace: my-namespace
  monitoring: {}
```

### Custom detection settings

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: AppFirewall
metadata:
  name: custom-waf
spec:
  xcNamespace: my-namespace
  blocking: {}
  detectionSettings:
    signature_selection_setting:
      high_medium_low_accuracy_signatures: {}
    enable_threat_campaigns: {}
    enable_suppression: {}
```

### Attach to an HTTPLoadBalancer

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: protected-app
spec:
  xcNamespace: my-namespace
  # ...
  appFirewall:
    name: my-waf
```

## Spec Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |

### Enforcement Mode (choose one)

| Field | Description |
|-------|-------------|
| `monitoring` | `{}` to log but not block violations |
| `blocking` | `{}` to actively block violations |

### Detection Settings (choose one)

| Field | Description |
|-------|-------------|
| `defaultDetectionSettings` | `{}` for F5 XC defaults |
| `detectionSettings` | Custom detection config |

### Additional OneOf Groups

| Group | Options |
|-------|---------|
| Blocking page | `useDefaultBlockingPage`, `blockingPage` |
| Response codes | `allowAllResponseCodes`, `allowedResponseCodes` |
| Bot protection | `defaultBotSetting`, `botProtectionSetting` |
| Anonymization | `defaultAnonymization`, `customAnonymization` |

All OneOf fields accept JSON objects passed through to the XC API. See the [API reference](https://docs.cloud.f5.com/docs-v2/api/app-firewall) for available sub-fields.
