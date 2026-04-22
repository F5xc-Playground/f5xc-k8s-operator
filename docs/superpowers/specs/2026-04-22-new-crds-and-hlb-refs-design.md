# New CRDs and HTTP LB References Design

## Overview

Add three new standalone CRDs — **APIDefinition**, **UserIdentification**, **MaliciousUserMitigation** — each with the full operator stack (types, xcclient CRUD, controller/reconciler, mapper, tests). Then update the HTTP LB CRD to reference these resources via ObjectRef fields.

## Motivation

The HTTP LB already supports inline configuration for API discovery, challenge policies, and user ID (client IP only). These three new resources are standalone XC objects with their own lifecycle — users create them once and reference them from multiple LBs. The operator should manage them independently.

## Scope

- 3 new CRDs with full CRUD lifecycle
- HTTP LB types/mapper/xcclient updates for ObjectRef references
- Contract tests against real F5 XC tenant
- All fields fully typed (no `apiextensionsv1.JSON`) per project standard

---

## CRD 1: APIDefinition

**XC API path:** `POST /api/config/namespaces/{ns}/api_definitions`
**XC resource name:** `api_definitions`
**Short name:** `apidef`

### Spec

```go
type APIDefinitionSpec struct {
    // +kubebuilder:validation:Required
    XCNamespace               string         `json:"xcNamespace"`
    SwaggerSpecs              []string       `json:"swaggerSpecs,omitempty"`
    APIInventoryInclusionList []APIOperation `json:"apiInventoryInclusionList,omitempty"`
    APIInventoryExclusionList []APIOperation `json:"apiInventoryExclusionList,omitempty"`
    NonAPIEndpoints           []APIOperation `json:"nonAPIEndpoints,omitempty"`

    // OneOf: schema_updates_strategy
    MixedSchemaOrigin  *EmptyObject `json:"mixedSchemaOrigin,omitempty"`
    StrictSchemaOrigin *EmptyObject `json:"strictSchemaOrigin,omitempty"`
}

type APIOperation struct {
    Method string `json:"method"`
    Path   string `json:"path"`
}
```

### Status

Standard status: `Conditions`, `ObservedGeneration`, `XCResourceVersion`, `XCUID`, `XCNamespace`.

### Wire-format types (mapper)

```go
type xcAPIDefinitionSpec struct {
    SwaggerSpecs              []string        `json:"swagger_specs,omitempty"`
    APIInventoryInclusionList []xcAPIOperation `json:"api_inventory_inclusion_list,omitempty"`
    APIInventoryExclusionList []xcAPIOperation `json:"api_inventory_exclusion_list,omitempty"`
    NonAPIEndpoints           []xcAPIOperation `json:"non_api_endpoints,omitempty"`
    MixedSchemaOrigin         *EmptyObject     `json:"mixed_schema_origin,omitempty"`
    StrictSchemaOrigin        *EmptyObject     `json:"strict_schema_origin,omitempty"`
}

type xcAPIOperation struct {
    Method string `json:"method"`
    Path   string `json:"path"`
}
```

### XC Client

```go
// In xcclient/types.go
ResourceAPIDefinition = "api_definitions"

// New file: xcclient/apidefinition.go
// Standard CRUD: Create, Get, Replace, Delete, List
```

---

## CRD 2: UserIdentification

**XC API path:** `POST /api/config/namespaces/{ns}/user_identifications`
**XC resource name:** `user_identifications`
**Short name:** `uid`

### Spec

```go
type UserIdentificationSpec struct {
    // +kubebuilder:validation:Required
    XCNamespace string                   `json:"xcNamespace"`
    Rules       []UserIdentificationRule `json:"rules"`
}

// Each rule is a OneOf — exactly one identifier field set per rule.
// Min 1, max 4 rules. Evaluated sequentially.
type UserIdentificationRule struct {
    None                   *EmptyObject `json:"none,omitempty"`
    ClientIP               *EmptyObject `json:"clientIP,omitempty"`
    ClientASN              *EmptyObject `json:"clientASN,omitempty"`
    ClientCity             *EmptyObject `json:"clientCity,omitempty"`
    ClientCountry          *EmptyObject `json:"clientCountry,omitempty"`
    ClientRegion           *EmptyObject `json:"clientRegion,omitempty"`
    CookieName             string       `json:"cookieName,omitempty"`
    HTTPHeaderName         string       `json:"httpHeaderName,omitempty"`
    IPAndHTTPHeaderName    string       `json:"ipAndHTTPHeaderName,omitempty"`
    IPAndTLSFingerprint    *EmptyObject `json:"ipAndTLSFingerprint,omitempty"`
    IPAndJA4TLSFingerprint *EmptyObject `json:"ipAndJA4TLSFingerprint,omitempty"`
    TLSFingerprint         *EmptyObject `json:"tlsFingerprint,omitempty"`
    JA4TLSFingerprint      *EmptyObject `json:"ja4TLSFingerprint,omitempty"`
    JWTClaimName           string       `json:"jwtClaimName,omitempty"`
    QueryParamKey          string       `json:"queryParamKey,omitempty"`
}
```

### Status

Standard status: `Conditions`, `ObservedGeneration`, `XCResourceVersion`, `XCUID`, `XCNamespace`.

### Wire-format types (mapper)

```go
type xcUserIdentificationSpec struct {
    Rules []xcUserIdentificationRule `json:"rules"`
}

type xcUserIdentificationRule struct {
    None                   *EmptyObject `json:"none,omitempty"`
    ClientIP               *EmptyObject `json:"client_ip,omitempty"`
    ClientASN              *EmptyObject `json:"client_asn,omitempty"`
    ClientCity             *EmptyObject `json:"client_city,omitempty"`
    ClientCountry          *EmptyObject `json:"client_country,omitempty"`
    ClientRegion           *EmptyObject `json:"client_region,omitempty"`
    CookieName             string       `json:"cookie_name,omitempty"`
    HTTPHeaderName         string       `json:"http_header_name,omitempty"`
    IPAndHTTPHeaderName    string       `json:"ip_and_http_header_name,omitempty"`
    IPAndTLSFingerprint    *EmptyObject `json:"ip_and_tls_fingerprint,omitempty"`
    IPAndJA4TLSFingerprint *EmptyObject `json:"ip_and_ja4_tls_fingerprint,omitempty"`
    TLSFingerprint         *EmptyObject `json:"tls_fingerprint,omitempty"`
    JA4TLSFingerprint      *EmptyObject `json:"ja4_tls_fingerprint,omitempty"`
    JWTClaimName           string       `json:"jwt_claim_name,omitempty"`
    QueryParamKey          string       `json:"query_param_key,omitempty"`
}
```

### XC Client

```go
// In xcclient/types.go
ResourceUserIdentification = "user_identifications"

// New file: xcclient/useridentification.go
// Standard CRUD: Create, Get, Replace, Delete, List
```

---

## CRD 3: MaliciousUserMitigation

**XC API path:** `POST /api/config/namespaces/{ns}/malicious_user_mitigations`
**XC resource name:** `malicious_user_mitigations`
**Short name:** `mum`

### Spec

```go
type MaliciousUserMitigationSpec struct {
    // +kubebuilder:validation:Required
    XCNamespace    string                       `json:"xcNamespace"`
    MitigationType *MaliciousUserMitigationType `json:"mitigationType,omitempty"`
}

type MaliciousUserMitigationType struct {
    Rules []MaliciousUserMitigationRule `json:"rules"`
}

// Max 3 rules, one per threat level (low/medium/high)
type MaliciousUserMitigationRule struct {
    ThreatLevel      MaliciousUserThreatLevel      `json:"threatLevel"`
    MitigationAction MaliciousUserMitigationAction `json:"mitigationAction"`
}

// OneOf — exactly one set
type MaliciousUserThreatLevel struct {
    Low    *EmptyObject `json:"low,omitempty"`
    Medium *EmptyObject `json:"medium,omitempty"`
    High   *EmptyObject `json:"high,omitempty"`
}

// OneOf — exactly one set
type MaliciousUserMitigationAction struct {
    BlockTemporarily    *EmptyObject `json:"blockTemporarily,omitempty"`
    CaptchaChallenge    *EmptyObject `json:"captchaChallenge,omitempty"`
    JavascriptChallenge *EmptyObject `json:"javascriptChallenge,omitempty"`
}
```

### Status

Standard status: `Conditions`, `ObservedGeneration`, `XCResourceVersion`, `XCUID`, `XCNamespace`.

### Wire-format types (mapper)

```go
type xcMaliciousUserMitigationSpec struct {
    MitigationType *xcMaliciousUserMitigationType `json:"mitigation_type,omitempty"`
}

type xcMaliciousUserMitigationType struct {
    Rules []xcMaliciousUserMitigationRule `json:"rules"`
}

type xcMaliciousUserMitigationRule struct {
    ThreatLevel      xcMaliciousUserThreatLevel      `json:"threat_level"`
    MitigationAction xcMaliciousUserMitigationAction `json:"mitigation_action"`
}

type xcMaliciousUserThreatLevel struct {
    Low    *EmptyObject `json:"low,omitempty"`
    Medium *EmptyObject `json:"medium,omitempty"`
    High   *EmptyObject `json:"high,omitempty"`
}

type xcMaliciousUserMitigationAction struct {
    BlockTemporarily    *EmptyObject `json:"block_temporarily,omitempty"`
    CaptchaChallenge    *EmptyObject `json:"captcha_challenge,omitempty"`
    JavascriptChallenge *EmptyObject `json:"javascript_challenge,omitempty"`
}
```

### XC Client

```go
// In xcclient/types.go
ResourceMaliciousUserMitigation = "malicious_user_mitigations"

// New file: xcclient/malicioususermitigation.go
// Standard CRUD: Create, Get, Replace, Delete, List
```

---

## HTTP LB Updates

### 1. User Identification — expand `user_id_choice` OneOf

**In `httplb_types.go` HTTPLoadBalancerSpec:**

```go
// User ID OneOf (existing UserIDClientIP + new UserIdentification)
UserIDClientIP     *EmptyObject `json:"userIDClientIP,omitempty"`
UserIdentification *ObjectRef   `json:"userIdentification,omitempty"`
```

**In `xcclient/httplb.go` HTTPLoadBalancerSpec:**

```go
UserIDClientIP     json.RawMessage `json:"user_id_client_ip,omitempty"`
UserIdentification *ObjectRef      `json:"user_identification,omitempty"`
```

**Mapper:** If `spec.UserIdentification != nil`, map to `out.UserIdentification = mapObjectRefPtr(spec.UserIdentification)`.

### 2. API Definition — new `api_definition_choice` OneOf

**New types in `httplb_types.go`:**

```go
// API definition OneOf
DisableAPIDefinition *EmptyObject           `json:"disableAPIDefinition,omitempty"`
APISpecification     *APISpecificationConfig `json:"apiSpecification,omitempty"`

type APISpecificationConfig struct {
    APIDefinition              *ObjectRef            `json:"apiDefinition"`
    // Validation OneOf
    ValidationDisabled         *EmptyObject          `json:"validationDisabled,omitempty"`
    ValidationAllSpecEndpoints *EmptyObject          `json:"validationAllSpecEndpoints,omitempty"`
    ValidationCustomList       *ValidationCustomList `json:"validationCustomList,omitempty"`
}

type ValidationCustomList struct {
    EndpointValidationList []APIOperation `json:"endpointValidationList,omitempty"`
}
```

Note: `APIOperation` is reused from the APIDefinition CRD types — it lives in `shared_types.go`.

**In `xcclient/httplb.go` HTTPLoadBalancerSpec:**

```go
DisableAPIDefinition json.RawMessage `json:"disable_api_definition,omitempty"`
APISpecification     json.RawMessage `json:"api_specification,omitempty"`
```

**Wire-format types in mapper:**

```go
type xcAPISpecificationConfig struct {
    APIDefinition              *xcObjectRef            `json:"api_definition"`
    ValidationDisabled         *EmptyObject            `json:"validation_disabled,omitempty"`
    ValidationAllSpecEndpoints *EmptyObject            `json:"validation_all_spec_endpoints,omitempty"`
    ValidationCustomList       *xcValidationCustomList `json:"validation_custom_list,omitempty"`
}

type xcValidationCustomList struct {
    EndpointValidationList []xcAPIOperation `json:"endpoint_validation_list,omitempty"`
}
```

### 3. Malicious User Detection — new top-level toggle OneOf

**In `httplb_types.go` HTTPLoadBalancerSpec:**

```go
// Malicious user detection OneOf
DisableMaliciousUserDetection *EmptyObject `json:"disableMaliciousUserDetection,omitempty"`
EnableMaliciousUserDetection  *EmptyObject `json:"enableMaliciousUserDetection,omitempty"`
```

**In `xcclient/httplb.go` HTTPLoadBalancerSpec:**

```go
DisableMaliciousUserDetection json.RawMessage `json:"disable_malicious_user_detection,omitempty"`
EnableMaliciousUserDetection  json.RawMessage `json:"enable_malicious_user_detection,omitempty"`
```

**Mapper:** Both map to `emptyObjectJSON`.

### 4. Malicious User Mitigation — add ObjectRef inside PolicyBasedChallengeConfig

**In `httplb_types.go` PolicyBasedChallengeConfig (existing struct):**

```go
// Add alongside existing DefaultMitigationSettings and MaliciousUserMitigationBypass:
MaliciousUserMitigation *ObjectRef `json:"maliciousUserMitigation,omitempty"`
```

**In mapper xcPolicyBasedChallengeConfig:**

```go
MaliciousUserMitigation *xcObjectRef `json:"malicious_user_mitigation,omitempty"`
```

**Mapper:** `mapObjectRefPtr(pbc.MaliciousUserMitigation)`.

---

## Shared Types

`APIOperation` is used by both the APIDefinition CRD and the HTTP LB's `ValidationCustomList`. It belongs in `shared_types.go`.

---

## File Inventory

### New Files

| File | Purpose |
|------|---------|
| `api/v1alpha1/apidefinition_types.go` | APIDefinition CRD types |
| `api/v1alpha1/useridentification_types.go` | UserIdentification CRD types |
| `api/v1alpha1/malicioususermitigation_types.go` | MaliciousUserMitigation CRD types |
| `internal/xcclient/apidefinition.go` | APIDefinition xcclient CRUD |
| `internal/xcclient/useridentification.go` | UserIdentification xcclient CRUD |
| `internal/xcclient/malicioususermitigation.go` | MaliciousUserMitigation xcclient CRUD |
| `internal/controller/apidefinition_controller.go` | APIDefinition reconciler |
| `internal/controller/useridentification_controller.go` | UserIdentification reconciler |
| `internal/controller/malicioususermitigation_controller.go` | MaliciousUserMitigation reconciler |
| `internal/controller/apidefinition_mapper.go` | APIDefinition mapper + wire types |
| `internal/controller/useridentification_mapper.go` | UserIdentification mapper + wire types |
| `internal/controller/malicioususermitigation_mapper.go` | MaliciousUserMitigation mapper + wire types |
| `internal/controller/apidefinition_mapper_test.go` | APIDefinition mapper tests |
| `internal/controller/useridentification_mapper_test.go` | UserIdentification mapper tests |
| `internal/controller/malicioususermitigation_mapper_test.go` | MaliciousUserMitigation mapper tests |

### Modified Files

| File | Change |
|------|--------|
| `api/v1alpha1/shared_types.go` | Add `APIOperation` type |
| `api/v1alpha1/httplb_types.go` | Add ObjectRef fields, APISpecificationConfig, ValidationCustomList, detection toggle |
| `internal/xcclient/httplb.go` | Add matching fields |
| `internal/xcclient/types.go` | Add 3 resource constants |
| `internal/controller/httplb_mapper.go` | Add mapper logic for new fields + wire types |
| `internal/controller/httplb_mapper_test.go` | Test new field mappings |
| `internal/controller/contract_test.go` | Add contract tests for 3 new resources |
| `internal/xcclientset/clientset.go` | Register 3 new resource types |
| `cmd/main.go` | Register 3 new reconcilers |

---

## Testing Strategy

- **Mapper unit tests:** Each new CRD mapper gets tests covering all OneOf branches. HTTP LB mapper tests extended for new fields.
- **Contract tests:** Each new resource gets a create/get/replace/delete cycle against real XC tenant (build tag: `contract`).
- **Code generation:** `make generate` and `make manifests` after all type changes.
