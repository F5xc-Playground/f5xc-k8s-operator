# MaliciousUserMitigation

A MaliciousUserMitigation defines what actions to take against users identified as malicious at different threat levels (low, medium, high). It is referenced from an HTTPLoadBalancer's `policyBasedChallenge` configuration via the `maliciousUserMitigation` field.

**Short name:** `mum`

## F5 XC Documentation

- [Malicious User Mitigation (How-To)](https://docs.cloud.f5.com/docs/how-to/app-security/malicious-user)
- [Malicious User Mitigation API Reference](https://docs.cloud.f5.com/docs-v2/api/malicious-user-mitigation)

## Examples

### All threat levels

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: MaliciousUserMitigation
metadata:
  name: standard-mitigation
spec:
  xcNamespace: my-namespace
  mitigationType:
    rules:
      - threatLevel:
          low: {}
        mitigationAction:
          javascriptChallenge: {}
      - threatLevel:
          medium: {}
        mitigationAction:
          captchaChallenge: {}
      - threatLevel:
          high: {}
        mitigationAction:
          blockTemporarily: {}
```

### High-threat only

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: MaliciousUserMitigation
metadata:
  name: block-high-only
spec:
  xcNamespace: my-namespace
  mitigationType:
    rules:
      - threatLevel:
          high: {}
        mitigationAction:
          blockTemporarily: {}
```

### Reference from an HTTPLoadBalancer

Malicious user mitigation is referenced inside the `policyBasedChallenge` config. You also need to enable malicious user detection at the top level.

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: HTTPLoadBalancer
metadata:
  name: my-app
spec:
  xcNamespace: my-namespace
  # ...
  enableMaliciousUserDetection: {}
  policyBasedChallenge:
    maliciousUserMitigation:
      name: standard-mitigation
```

## Spec Reference

> **Full field reference:** [Malicious User Mitigation API Documentation](https://docs.cloud.f5.com/docs-v2/api/malicious-user-mitigation)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `xcNamespace` | string | Yes | F5 XC namespace |
| `mitigationType` | MaliciousUserMitigationType | No | Mitigation rules by threat level |

### MaliciousUserMitigationType

| Field | Type | Description |
|-------|------|-------------|
| `rules` | []MaliciousUserMitigationRule | Max 3 rules, one per threat level |

### MaliciousUserMitigationRule

| Field | Type | Description |
|-------|------|-------------|
| `threatLevel` | ThreatLevel | Which threat level this rule applies to |
| `mitigationAction` | MitigationAction | What action to take |

### ThreatLevel (choose one)

| Field | Description |
|-------|-------------|
| `low` | `{}` for low-threat users |
| `medium` | `{}` for medium-threat users |
| `high` | `{}` for high-threat users |

### MitigationAction (choose one)

| Field | Description |
|-------|-------------|
| `blockTemporarily` | `{}` to temporarily block the user |
| `captchaChallenge` | `{}` to present a CAPTCHA challenge |
| `javascriptChallenge` | `{}` to present a JavaScript challenge |
