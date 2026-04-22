# F5 XC Operator — OAS Structural Audit

**Date:** 2026-04-21  
**Status:** All breaking issues fixed.  
**Specs fetched from:** `https://git.k11s.io/k11s-home/claus-docs/raw/branch/main/Research/f5-xc/oas/`  
**Severity legend:**
- **BREAKING** — API will reject the request or silently ignore a required field
- **COSMETIC** — Works today but naming/structure deviates from the spec
- **ENHANCEMENT** — Optional fields the API supports that we don't expose

---

## 1. RateLimiter

**OAS file:** `docs-cloud-f5-com.0190.public.ves.io.schema.rate_limiter.ves-swagger.json`

### What we send today

```json
{
  "spec": {
    "total_number": 100,
    "unit": "MINUTE",
    "burst_multiplier": 5
  }
}
```

### What the API expects

```json
{
  "spec": {
    "limits": [
      {
        "total_number": 100,
        "unit": "MINUTE",
        "burst_multiplier": 5
      }
    ]
  }
}
```

The top-level spec has **two** fields: `limits` (array, maxItems: 1, **required**) and `user_identification` (array, optional). The rate limit values live inside a `rate_limiterRateLimitValue` object inside `limits[]`. There are no flat top-level `total_number`, `unit`, or `burst_multiplier` fields on the spec.

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/xcratelimiter.go` `XCRateLimiterSpec` | Struct sends `total_number`, `unit`, `burst_multiplier` as flat fields on spec. API requires them nested inside `limits[0]`. |
| **BREAKING** | `api/v1alpha1/ratelimiter_types.go` `RateLimiterSpec` | CRD exposes `threshold`, `unit`, `burstMultiplier` flat. The field name `Threshold` also maps to `total_number` correctly in the client, but the wrapping is wrong. |
| **BREAKING** | `internal/controller/ratelimiter_mapper.go` `mapRateLimiterSpec()` | Builds `XCRateLimiterSpec` with flat fields; must wrap in `limits: [{...}]` instead. |
| ENHANCEMENT | `XCRateLimiterSpec` | `period_multiplier` (uint32, optional) — combined with `unit` for sub-unit durations — is not exposed. |
| ENHANCEMENT | `XCRateLimiterSpec` | `action_block` / `disabled` oneOf for what happens when limit is exceeded is not exposed. |
| ENHANCEMENT | `XCRateLimiterSpec` | `leaky_bucket` / `token_bucket` algorithm choice is not exposed. |
| ENHANCEMENT | `XCRateLimiterCreate` | `user_identification` array ref to a `user_identification` policy object is not exposed. |

### Required changes

**`internal/xcclient/xcratelimiter.go`**
```go
type RateLimitValue struct {
    TotalNumber     uint32 `json:"total_number,omitempty"`
    Unit            string `json:"unit,omitempty"`
    BurstMultiplier uint32 `json:"burst_multiplier,omitempty"`
}

type XCRateLimiterSpec struct {
    Limits []RateLimitValue `json:"limits"`
}
```

**`internal/controller/ratelimiter_mapper.go`**
```go
func mapRateLimiterSpec(spec *v1alpha1.RateLimiterSpec) xcclient.XCRateLimiterSpec {
    val := xcclient.RateLimitValue{
        TotalNumber: spec.Threshold,
        Unit:        spec.Unit,
    }
    if spec.BurstMultiplier != nil {
        val.BurstMultiplier = *spec.BurstMultiplier
    }
    return xcclient.XCRateLimiterSpec{
        Limits: []xcclient.RateLimitValue{val},
    }
}
```

---

## 2. OriginPool

**OAS file:** `docs-cloud-f5-com.0177.public.ves.io.schema.views.origin_pool.ves-swagger.json`  
**Spec type:** `viewsorigin_poolCreateSpecType`

### What the API expects (key structural fields)

```
origin_servers[]     — array of origin_poolOriginServerType objects (required)
port                 — integer, mutually exclusive with automatic_port / lb_port
no_tls               — empty object (mutually exclusive with use_tls)
use_tls              — origin_poolUpstreamTlsParameters object
loadbalancer_algorithm — string enum: ROUND_ROBIN | LEAST_REQUEST | RING_HASH | RANDOM | LB_OVERRIDE
healthcheck[]        — array of schemaviewsObjectRefType ({name, namespace, tenant})
```

### Origin server type differences

The API's `origin_poolOriginServerType` has `labels` (map) and additional server types (`cbip_service`, `custom_endpoint_object`, `vn_private_ip`, `vn_private_name`) that we don't expose — those are enhancements. The fields we do map have different sub-structures:

| Server type | Our field | API field | Severity |
|-------------|-----------|-----------|----------|
| `PrivateIP` | `site` (ObjectRef) | `site_locator` (viewsSiteLocator) + `inside_network`/`outside_network`/`segment` choice | BREAKING |
| `PrivateName` | `site` (ObjectRef) | `site_locator` (viewsSiteLocator) + network choice | BREAKING |
| `K8SService` | `site` (ObjectRef) + `serviceNamespace` | `site_locator` + `service_name` (format: `svc.ns:cluster-id`) + network choice | BREAKING |
| `ConsulService` | `site` (ObjectRef) + `serviceName` | `site_locator` + `service_name` (format: `svc:cluster-id`) + network choice | BREAKING |
| `PublicIP` | `ip` | `ip` | OK |
| `PublicName` | `dns_name` | `dns_name` | OK |

**The `site` field name**: Our xcclient sends `"site"` but the API field is `"site_locator"` (a `viewsSiteLocator` object, not a plain ObjectRef). This is a field-name mismatch on all private/k8s/consul origin server types.

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/originpool.go` `PrivateIP`, `PrivateName`, `K8SService`, `ConsulService` | Field `site` maps to JSON `"site"` but API expects `"site_locator"` (a `viewsSiteLocator` wrapper). |
| **BREAKING** | Same structs | Network choice (`inside_network` / `outside_network` / `segment`) is absent. The API requires one of these oneOf fields alongside `site_locator`. |
| **BREAKING** | `internal/xcclient/originpool.go` `K8SService` | `service_namespace` is sent as a separate field but the API embeds namespace in `service_name` as `"servicename.namespace:cluster-id"` format. |
| COSMETIC | `internal/xcclient/originpool.go` `OriginPoolSpec` | `LoadBalancerAlgorithm` field name is correct (`loadbalancer_algorithm`). Enum values must be uppercase: `ROUND_ROBIN`, `LEAST_REQUEST`, `RING_HASH`, `RANDOM`, `LB_OVERRIDE`. Verify CRD validation enforces this. |
| COSMETIC | `internal/xcclient/originpool.go` `OriginPoolSpec` | `HealthCheck` field is named `healthcheck` (JSON) which matches the API. Structure (array of ObjectRef with `name`/`namespace`/`tenant`) also matches. OK. |
| ENHANCEMENT | `OriginServer` | `labels` map on each origin server (for subset routing) not exposed. |
| ENHANCEMENT | `OriginPoolSpec` | `advanced_options` (circuit breaker, outlier detection, etc.) not exposed. |
| ENHANCEMENT | `OriginPoolSpec` | `endpoint_selection` policy not exposed. |
| ENHANCEMENT | `origin_poolOriginServerType` | `cbip_service`, `custom_endpoint_object`, `vn_private_ip`, `vn_private_name` server types not supported. |

### `viewsSiteLocator` structure
The `site_locator` field is a oneOf wrapper — it is NOT a plain ObjectRef. The API schema wraps it:
```json
{
  "site_locator": {
    "site": { "name": "...", "namespace": "..." }
  }
}
```
or `"virtual_site"` instead of `"site"`. This must be implemented as a proper struct.

---

## 3. HTTPLoadBalancer

**OAS file:** `docs-cloud-f5-com.0073.public.ves.io.schema.views.http_loadbalancer.ves-swagger.json`  
**Spec type:** `viewshttp_loadbalancerCreateSpecType`

### Overall assessment

The HTTP LB mapper uses `json.RawMessage` pass-through for all OneOf fields, which means field names flow directly from the user's YAML. The risk here is that the CRD field names (camelCase) must be translated correctly to the snake_case API names by the mapper. The mapper does this correctly for all OneOf groups.

### Specific findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/httplb.go` `HTTPLoadBalancerSpec.Routes` | Our `Routes` field is `json.RawMessage` with tag `"routes"`. The API field is also `"routes"` (array of `viewshttp_loadbalancerRouteType`). This is correct for pass-through, but the routes array items must conform to the API's route type — this is opaque to us and only breaks if callers supply invalid route objects. |
| **BREAKING** | `internal/xcclient/httplb.go` `HTTPLoadBalancerSpec` | The `user_identification` field in the API is a plain `ObjectRef` (`"user_identification": {"name":"...", "namespace":"..."}`), but we have `user_id_client_ip` as a separate field. These are different: `user_id_client_ip` is an empty-object flag saying "identify by client IP", while `user_identification` is a ref to a `user_identification` policy object. We currently only expose `user_id_client_ip` (correct), but `user_identification` (the policy object ref) is missing as an enhancement. |
| COSMETIC | `api/v1alpha1/httplb_types.go` `HTTPSAutoCert` | CRD field `httpsAutoCert` maps to `https_auto_cert` in the mapper — correct. |
| COSMETIC | `internal/xcclient/httplb.go` | All OneOf field names match the OAS spec exactly. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `add_location` (boolean) — appends x-volterra-location header — not exposed. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `user_identification` (ObjectRef to user_identification policy) not exposed; only the `user_id_client_ip` choice is. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `cors_policy`, `csrf_policy`, `jwt_validation`, `api_protection_rules`, `api_rate_limit` not exposed. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `more_option` (advanced options) not exposed. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `multi_lb_app` / `single_lb_app` not exposed. |
| ENHANCEMENT | `HTTPLoadBalancerSpec` | `default_pool` and `default_pool_list` as alternatives to `default_route_pools` not exposed. |

### `default_route_pools` structure

The OAS defines `default_route_pools` items as `viewsOriginPoolWithWeight` with fields: `pool` (ObjectRef, mutually exclusive with `cluster`), `cluster` (ObjectRef), `weight` (integer), `priority` (integer), `endpoint_subsets` (map). Our `RoutePool` struct sends `pool` + `weight` + `priority` which is correct. The `endpoint_subsets` field is an enhancement.

---

## 4. TCPLoadBalancer

**OAS file:** `docs-cloud-f5-com.0078.public.ves.io.schema.views.tcp_loadbalancer.ves-swagger.json`  
**Spec type:** `viewstcp_loadbalancerCreateSpecType`

### TLS field name mismatches — BREAKING

The API's TLS oneOf fields for TCP LB are **different** from what we send:

| Our field (JSON tag) | API field name | Issue |
|----------------------|----------------|-------|
| `no_tls` | **`tcp`** (empty object) | **BREAKING** — wrong field name |
| `tls_parameters` | **`tls_tcp`** (object) | **BREAKING** — wrong field name |
| `tls_tcp_passthrough` (our TLSPassthrough) | No equivalent; `tls_tcp_auto_cert` exists for auto-cert TLS | **BREAKING** — TLS passthrough is not a concept in TCP LB; `tls_tcp_auto_cert` is the auto-cert variant |

The OAS has three `loadbalancer_type` choices: `tcp` (no TLS), `tls_tcp` (BYOC TLS), `tls_tcp_auto_cert` (auto-managed certs). There is no "TLS passthrough" concept for TCP LB.

### Origin pools field name mismatch — BREAKING

| Our field (JSON tag) | API field name | Issue |
|----------------------|----------------|-------|
| `origin_pools` | **`origin_pools_weights`** | **BREAKING** — wrong field name |

Our `TCPLoadBalancerSpec` sends `"origin_pools"` but the API expects `"origin_pools_weights"`.

### Additional TCP LB findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/tcplb.go` `TCPLoadBalancerSpec` | JSON tag `"no_tls"` should be `"tcp"`. |
| **BREAKING** | `internal/xcclient/tcplb.go` `TCPLoadBalancerSpec` | JSON tag `"tls_parameters"` should be `"tls_tcp"`. |
| **BREAKING** | `internal/xcclient/tcplb.go` `TCPLoadBalancerSpec` | JSON tag `"tls_tcp_passthrough"` should be `"tls_tcp_auto_cert"` (or this field needs re-evaluation; TLS passthrough is not in the API). |
| **BREAKING** | `internal/xcclient/tcplb.go` `TCPLoadBalancerSpec` | JSON tag `"origin_pools"` should be `"origin_pools_weights"`. |
| **BREAKING** | `api/v1alpha1/tcplb_types.go` `TCPLoadBalancerSpec` | CRD field `tlsPassthrough` with tag `"tlsPassthrough"` needs to map to `tls_tcp_auto_cert` or be renamed to `tlsTCPAutoCert`. |
| **BREAKING** | `api/v1alpha1/tcplb_types.go` `TCPLoadBalancerSpec` | CRD field `noTLS` with tag `"noTLS"` maps to `"tcp"` in the API (not `"no_tls"`). |
| ENHANCEMENT | `TCPLoadBalancerSpec` | `hash_policy_choice_round_robin`, `hash_policy_choice_least_active`, `hash_policy_choice_random`, `hash_policy_choice_source_ip_stickiness` LB algorithm oneOf fields not exposed. |
| ENHANCEMENT | `TCPLoadBalancerSpec` | `active_service_policies`, `service_policies_from_namespace`, `no_service_policies` fields not exposed. |
| ENHANCEMENT | `TCPLoadBalancerSpec` | `idle_timeout`, `dns_volterra_managed` not exposed. |

---

## 5. AppFirewall

**OAS file:** `docs-cloud-f5-com.0019.public.ves.io.schema.app_firewall.ves-swagger.json`  
**Spec type:** `app_firewallCreateSpecType`

### Overall assessment

Field names largely match. One missing field in our anonymization group.

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/appfirewall.go` `AppFirewallSpec` | `disable_anonymization` is a valid third option in the anonymization oneOf (alongside `default_anonymization` and `custom_anonymization`) but is absent from our struct and CRD. If a user needs to disable anonymization entirely, there's no way to express it. |
| **BREAKING** | `internal/xcclient/appfirewall.go` `AppFirewallSpec` | `use_loadbalancer_setting` field is in our code but does **not** appear in the OAS `app_firewallCreateSpecType`. This field likely does not exist on the standalone AppFirewall resource — it may be a field on the HTTPLoadBalancer's WAF inline config. Sending it may cause the API to reject or ignore the request. |
| ENHANCEMENT | `AppFirewallSpec` | `ai_risk_based_blocking` — new detection mode option (third choice alongside `default_detection_settings` and `detection_settings`) not exposed. |

### Anonymization oneOf correction

OAS defines three choices:
- `default_anonymization` (empty object) ✓ we have this
- `disable_anonymization` (empty object) ✗ **missing from our code**
- `custom_anonymization` (AnonymizationSetting object) ✓ we have this

---

## 6. HealthCheck

**OAS file:** `docs-cloud-f5-com.0124.public.ves.io.schema.healthcheck.ves-swagger.json`  
**Spec type:** `healthcheckCreateSpecType`

### TCP health check field name mismatch — BREAKING

The API's `healthcheckTcpHealthCheck` fields are:
- `send_payload` (not `send`)
- `expected_response` (not `receive`)

Our code sends `send` and `receive`.

### HTTP health check field name mismatch — BREAKING

The API's `healthcheckHttpHealthCheck` has no `expected_status_codes` field with that exact structure; it uses `expected_status_codes` (array of strings) — this one is actually correct. However the field `use_http2` maps correctly.

| Our field | API field | Status |
|-----------|-----------|--------|
| `send` | `send_payload` | **BREAKING** — wrong field name |
| `receive` | `expected_response` | **BREAKING** — wrong field name |
| `path` | `path` | OK |
| `use_http2` | `use_http2` | OK |
| `expected_status_codes` | `expected_status_codes` | OK |

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/healthcheck.go` `TCPHealthCheck` | Field `Send string \`json:"send"\`` must be `json:"send_payload"`. |
| **BREAKING** | `internal/xcclient/healthcheck.go` `TCPHealthCheck` | Field `Receive string \`json:"receive"\`` must be `json:"expected_response"`. |
| **BREAKING** | `api/v1alpha1/healthcheck_types.go` `TCPHealthCheckSpec` | CRD fields `send` and `receive` are end-user facing; these are cosmetic in the CRD (user-chosen names are fine) but the xcclient JSON tags need updating. |
| ENHANCEMENT | `HTTPHealthCheck` / `HTTPHealthCheckSpec` | `headers` (map of strings) for custom request headers not exposed. |
| ENHANCEMENT | `HTTPHealthCheck` / `HTTPHealthCheckSpec` | `host_header` / `use_origin_server_name` oneOf for controlling Host header not exposed. |
| ENHANCEMENT | `HTTPHealthCheck` / `HTTPHealthCheckSpec` | `request_headers_to_remove` (array of strings) not exposed. |
| ENHANCEMENT | `HealthCheckSpec` | `udp_icmp_health_check` type not exposed. |

---

## 7. ServicePolicy

**OAS file:** `docs-cloud-f5-com.0208.public.ves.io.schema.service_policy.ves-swagger.json`  
**Spec type:** `schemaservice_policyCreateSpecType`

### The `algo` field does not exist in the API — BREAKING

Our `ServicePolicySpec` and `xcclient.ServicePolicySpec` both have an `algo` field (mapped to `"algo"` in JSON). This field is **not present** in either `schemaservice_policyCreateSpecType` or `schemaservice_policyReplaceSpecType`. Sending it will have no effect at best, and may cause a 400 response at worst depending on API strictness.

### `server_name` is a plain string, not a wrapped object

The OAS shows `server_name` as `type: string` (not an empty object/ObjectRef). Our code passes it as `json.RawMessage` from a `*apiextensionsv1.JSON`, which means the user would need to supply `{"serverName": "\"my-server\""}` (a JSON string literal) in the CRD — this is confusing and error-prone.

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| **BREAKING** | `internal/xcclient/servicepolicy.go` `ServicePolicySpec` | `Algo string \`json:"algo"\`` — field `algo` does not exist in the OAS. Remove or it risks request rejection. |
| **BREAKING** | `api/v1alpha1/servicepolicy_types.go` `ServicePolicySpec` | `Algo string \`json:"algo,omitempty"\`` — same issue at CRD level. |
| COSMETIC | `internal/xcclient/servicepolicy.go` | `server_name` is typed as `json.RawMessage` but the API expects a plain `string`. Should be `ServerName string \`json:"server_name,omitempty"\``. |
| ENHANCEMENT | `ServicePolicySpec` | `legacy_rule_list` field exists in `ReplaceSpecType` (not CreateSpecType) — not exposed. |
| ENHANCEMENT | `ServicePolicySpec` | Rule structure details (inside `rule_list`, `allow_list`, `deny_list`) are all passed as opaque `json.RawMessage` — acceptable for now but blocks validation. |

---

## 8. Certificate

**OAS file:** `docs-cloud-f5-com.0048.public.ves.io.schema.certificate.ves-swagger.json`  
**Spec type:** `certificateCreateSpecType`

### Overall assessment

Field names and structure largely match. There is one additional optional field we don't expose.

### Key structure verification

| Our field | API field | Status |
|-----------|-----------|--------|
| `CertificateURL` → `"certificate_url"` | `certificate_url` (string) | OK |
| `PrivateKey.ClearSecretInfo.URL` → nested `"clear_secret_info": {"url": "..."}` | `private_key.clear_secret_info.url` | OK |
| `PrivateKey.ClearSecretInfo.Provider` → `"provider"` | `private_key.clear_secret_info.provider` (optional) | OK |
| `CustomHashAlgorithms` | `custom_hash_algorithms` | OK |
| `DisableOcspStapling` | `disable_ocsp_stapling` | OK |
| `UseSystemDefaults` | `use_system_defaults` | OK |

The `string:///` + base64 URL format for `certificate_url` and `clear_secret_info.url` is the correct approach per the API spec.

The `schemaSecretType` for `private_key` supports both `clear_secret_info` and `blindfold_secret_info`. We only support `clear_secret_info` which is the right choice for operator-managed certs from k8s Secrets.

### Findings

| Severity | Location | Issue |
|----------|----------|-------|
| ENHANCEMENT | `internal/xcclient/certificate.go` `CertificateSpec` | `certificate_chain` (ObjectRef to a `certificate_chain` object) is not exposed. Needed for intermediate CA chain scenarios. |
| ENHANCEMENT | `internal/xcclient/certificate.go` `CertificatePrivKey` | `blindfold_secret_info` (for Volterra-managed secrets vault) not exposed — acceptable for k8s-operator use case. |

---

## Summary Table

| Resource | Breaking Issues | Cosmetic | Enhancements |
|----------|----------------|----------|--------------|
| RateLimiter | 3 (flat fields vs `limits[]` array, field name `threshold` vs `total_number`) | 0 | 4 |
| OriginPool | 4 (site_locator field name, network choice missing, K8SService namespace format) | 1 | 5 |
| HTTPLoadBalancer | 1 (user_identification vs user_id_client_ip distinction) | 0 | 6 |
| TCPLoadBalancer | 4 (no_tls→tcp, tls_parameters→tls_tcp, tls_passthrough concept wrong, origin_pools→origin_pools_weights) | 0 | 3 |
| AppFirewall | 2 (use_loadbalancer_setting doesn't exist on AppFirewall, disable_anonymization missing) | 0 | 1 |
| HealthCheck | 2 (send→send_payload, receive→expected_response) | 0 | 4 |
| ServicePolicy | 2 (algo field doesn't exist, server_name wrong type) | 1 | 2 |
| Certificate | 0 | 0 | 2 |

---

## Priority Fix Order

1. **RateLimiter** — completely broken; no creates/replaces will work
2. **TCPLoadBalancer** — completely broken due to wrong TLS and origin_pools field names
3. **HealthCheck** — TCP health checks broken (send/receive field names)
4. **OriginPool** — Private/K8S/Consul origin servers broken; public IP/name works
5. **AppFirewall** — `use_loadbalancer_setting` may cause 400s; disable_anonymization missing
6. **ServicePolicy** — `algo` field might cause 400s; server_name type mismatch
7. **HTTPLoadBalancer** — Mostly correct; minor user_identification gap
8. **Certificate** — No breaking issues
