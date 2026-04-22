# Typed CRD Fields Design

## Goal

Replace all `*apiextensionsv1.JSON` (opaque JSON) fields across every CRD with proper Go structs so users write native YAML instead of embedded JSON.

## Architecture

All new types go in `api/v1alpha1/shared_types.go` (for types shared across CRDs) or in the relevant `*_types.go` file (for CRD-specific types). The xcclient layer and controller mappers are updated to map between camelCase CRD fields and snake_case XC API fields.

Empty-object sentinels (`{}`) become `*struct{}` pointer fields — `nil` means absent, non-nil means set.

## Scope by CRD

### Certificate (3 fields)

| Current Field | Type Change | XC API Field |
|---|---|---|
| `disableOcspStapling` | `*struct{}` | `disable_ocsp_stapling` |
| `useSystemDefaults` | `*struct{}` | `use_system_defaults` |
| `customHashAlgorithms` | `*CustomHashAlgorithms` | `custom_hash_algorithms` |

```go
type CustomHashAlgorithms struct {
    HashAlgorithms []string `json:"hashAlgorithms"`
}
```

### OriginPool (10 fields)

| Current Field | Type Change | XC API Field |
|---|---|---|
| `noTLS` | `*struct{}` | `no_tls` |
| `useTLS` | `*OriginPoolTLS` | `use_tls` |
| `insideNetwork` (x4) | `*struct{}` | `inside_network` |
| `outsideNetwork` (x4) | `*struct{}` | `outside_network` |

```go
type OriginPoolTLS struct {
    // TLS config OneOf: defaultSecurity, lowSecurity, mediumSecurity, customSecurity
    DefaultSecurity *struct{}         `json:"defaultSecurity,omitempty"`
    LowSecurity     *struct{}         `json:"lowSecurity,omitempty"`
    MediumSecurity  *struct{}         `json:"mediumSecurity,omitempty"`
    CustomSecurity  *CustomTLSSecurity `json:"customSecurity,omitempty"`

    SNI                string `json:"sni,omitempty"`
    VolterraTrustedCA  *struct{} `json:"volterraTrustedCA,omitempty"`
    TrustedCAURL       string    `json:"trustedCAURL,omitempty"`
    DisableSNI         *struct{} `json:"disableSNI,omitempty"`
    UseServerVerification *struct{} `json:"useServerVerification,omitempty"`
    SkipServerVerification *struct{} `json:"skipServerVerification,omitempty"`
    NoMTLS             *struct{} `json:"noMTLS,omitempty"`
}

type CustomTLSSecurity struct {
    CipherSuites []string `json:"cipherSuites,omitempty"`
}
```

### AppFirewall (12 fields, 6 OneOf groups)

| Current Field | Type Change | XC API Field |
|---|---|---|
| `defaultDetectionSettings` | `*struct{}` | `default_detection_settings` |
| `detectionSettings` | `*DetectionSettings` | `detection_settings` |
| `monitoring` | `*struct{}` | `monitoring` |
| `blocking` | `*struct{}` | `blocking` |
| `useDefaultBlockingPage` | `*struct{}` | `use_default_blocking_page` |
| `blockingPage` | `*BlockingPage` | `blocking_page` |
| `allowAllResponseCodes` | `*struct{}` | `allow_all_response_codes` |
| `allowedResponseCodes` | `*AllowedResponseCodes` | `allowed_response_codes` |
| `defaultBotSetting` | `*struct{}` | `default_bot_setting` |
| `botProtectionSetting` | `*BotProtectionSetting` | `bot_protection_setting` |
| `defaultAnonymization` | `*struct{}` | `default_anonymization` |
| `disableAnonymization` | `*struct{}` | `disable_anonymization` |
| `customAnonymization` | `*CustomAnonymization` | `custom_anonymization` |

```go
type DetectionSettings struct {
    SignatureSelectionSetting *SignatureSelectionSetting `json:"signatureSelectionSetting,omitempty"`
    EnableSuppression        *struct{}                  `json:"enableSuppression,omitempty"`
    DisableSuppression       *struct{}                  `json:"disableSuppression,omitempty"`
    EnableThreatCampaigns    *struct{}                  `json:"enableThreatCampaigns,omitempty"`
    DisableThreatCampaigns   *struct{}                  `json:"disableThreatCampaigns,omitempty"`
}

type SignatureSelectionSetting struct {
    DefaultAttackTypeSettings *struct{} `json:"defaultAttackTypeSettings,omitempty"`
    AttackTypeSettings        *AttackTypeSettings `json:"attackTypeSettings,omitempty"`
    HighMediumLowAccuracySignatures *struct{} `json:"highMediumLowAccuracySignatures,omitempty"`
    HighMediumAccuracySignatures    *struct{} `json:"highMediumAccuracySignatures,omitempty"`
    OnlyHighAccuracySignatures      *struct{} `json:"onlyHighAccuracySignatures,omitempty"`
}

type AttackTypeSettings struct {
    DisabledAttackTypes []AttackType `json:"disabledAttackTypes,omitempty"`
}

type AttackType struct {
    Name string `json:"name"`
}

type BlockingPage struct {
    BlockingPage string `json:"blockingPage,omitempty"`
    ResponseCode string `json:"responseCode,omitempty"`
}

type AllowedResponseCodes struct {
    ResponseCode []int `json:"responseCode"`
}

type BotProtectionSetting struct {
    MaliciousBotAction string `json:"maliciousBotAction,omitempty"`
    SuspiciousBotAction string `json:"suspiciousBotAction,omitempty"`
    GoodBotAction string `json:"goodBotAction,omitempty"`
}

type CustomAnonymization struct {
    AnonymizationConfig []AnonymizationEntry `json:"anonymizationConfig,omitempty"`
    SpecificDomains     []string             `json:"specificDomains,omitempty"`
}

type AnonymizationEntry struct {
    HeaderName  string `json:"headerName,omitempty"`
    QueryParameter string `json:"queryParameter,omitempty"`
    CookieName  string `json:"cookieName,omitempty"`
}
```

### ServicePolicy (8 fields, 2 OneOf groups)

| Current Field | Type Change | XC API Field |
|---|---|---|
| `allowAllRequests` | `*struct{}` | `allow_all_requests` |
| `denyAllRequests` | `*struct{}` | `deny_all_requests` |
| `allowList` | `*PolicyAllowDenyList` | `allow_list` |
| `denyList` | `*PolicyAllowDenyList` | `deny_list` |
| `ruleList` | `*PolicyRuleList` | `rule_list` |
| `anyServer` | `*struct{}` | `any_server` |
| `serverNameMatcher` | `*ServerNameMatcher` | `server_name_matcher` |
| `serverSelector` | `*ServerSelector` | `server_selector` |

```go
type PolicyAllowDenyList struct {
    Prefixes              []string    `json:"prefixes,omitempty"`
    IPPrefixSet           []ObjectRef `json:"ipPrefixSet,omitempty"`
    ASNList               *ASNList    `json:"asnList,omitempty"`
    ASNSet                []ObjectRef `json:"asnSet,omitempty"`
    CountryList           []string    `json:"countryList,omitempty"`
    // Default action OneOf
    DefaultActionNextPolicy *struct{} `json:"defaultActionNextPolicy,omitempty"`
    DefaultActionDeny       *struct{} `json:"defaultActionDeny,omitempty"`
    DefaultActionAllow      *struct{} `json:"defaultActionAllow,omitempty"`
}

type ASNList struct {
    ASNumbers []uint32 `json:"asNumbers"`
}

type PolicyRuleList struct {
    Rules []PolicyRule `json:"rules"`
}

type PolicyRule struct {
    Metadata  map[string]string       `json:"metadata,omitempty"`
    Spec      *PolicyRuleSpec         `json:"spec,omitempty"`
}

type PolicyRuleSpec struct {
    Action      string          `json:"action,omitempty"`
    AnyClient   *struct{}       `json:"anyClient,omitempty"`
    ClientName  string          `json:"clientName,omitempty"`
    ClientSelector *LabelSelector `json:"clientSelector,omitempty"`
    IPMatcher   *IPMatcher      `json:"ipMatcher,omitempty"`
    ASNMatcher  *ASNMatcher     `json:"asnMatcher,omitempty"`
    // Path/headers/etc match conditions
    Path        *PathMatcher    `json:"path,omitempty"`
    Headers     []HeaderMatcher `json:"headers,omitempty"`
    HTTPMethod  *HTTPMethodMatcher `json:"httpMethod,omitempty"`
}

type ServerNameMatcher struct {
    ExactValues []string `json:"exactValues,omitempty"`
    RegexValues []string `json:"regexValues,omitempty"`
}

type ServerSelector struct {
    Expressions []string `json:"expressions,omitempty"`
}
```

### TCPLoadBalancer (7 fields, 2 OneOf groups)

| Current Field | Type Change | XC API Field |
|---|---|---|
| `noTLS` | `*struct{}` | `no_tls` |
| `tlsParameters` | `*TLSParameters` | `tls_parameters` |
| `tlsTCPAutoCert` | `*TLSTCPAutoCert` | `tls_tcp_auto_cert` |
| `advertiseOnPublicDefaultVIP` | `*struct{}` | `advertise_on_public_default_vip` |
| `advertiseOnPublic` | `*AdvertiseOnPublic` | `advertise_on_public` |
| `advertiseCustom` | `*AdvertiseCustom` | `advertise_custom` |
| `doNotAdvertise` | `*struct{}` | `do_not_advertise` |

Advertise types are shared with HTTPLoadBalancer (defined once in shared_types.go).

```go
type TLSParameters struct {
    TLSCertificates []TLSCertificateRef `json:"tlsCertificates,omitempty"`
    // TLS config OneOf
    DefaultSecurity *struct{}          `json:"defaultSecurity,omitempty"`
    LowSecurity     *struct{}          `json:"lowSecurity,omitempty"`
    MediumSecurity  *struct{}          `json:"mediumSecurity,omitempty"`
    CustomSecurity  *CustomTLSSecurity `json:"customSecurity,omitempty"`
    // MTLS OneOf
    NoMTLS  *struct{} `json:"noMTLS,omitempty"`
    UseMTLS *UseMTLS  `json:"useMTLS,omitempty"`
}

type TLSCertificateRef struct {
    CertificateURL string            `json:"certificateURL,omitempty"`
    PrivateKey     CertificatePrivKey `json:"privateKey,omitempty"`
}

type TLSTCPAutoCert struct {
    // MTLS OneOf
    NoMTLS  *struct{} `json:"noMTLS,omitempty"`
    UseMTLS *UseMTLS  `json:"useMTLS,omitempty"`
}

type UseMTLS struct {
    TrustedCAURL string `json:"trustedCAURL,omitempty"`
}

// Shared advertise types
type AdvertiseOnPublic struct {
    PublicIP *ObjectRef `json:"publicIP,omitempty"`
}

type AdvertiseCustom struct {
    AdvertiseWhere []AdvertiseWhere `json:"advertiseWhere"`
}

type AdvertiseWhere struct {
    Site        *AdvertiseSite    `json:"site,omitempty"`
    VirtualSite *AdvertiseSite    `json:"virtualSite,omitempty"`
    VirtualNetwork *AdvertiseVirtualNetwork `json:"virtualNetwork,omitempty"`
    Port        uint32            `json:"port,omitempty"`
    UseDefaultPort *struct{}      `json:"useDefaultPort,omitempty"`
}

type AdvertiseSite struct {
    Network string `json:"network,omitempty"`
    Site    ObjectRef `json:"site"`
    IPAddress string `json:"ipAddress,omitempty"`
}

type AdvertiseVirtualNetwork struct {
    DefaultVIP  *struct{} `json:"defaultVIP,omitempty"`
    SpecificVIP string    `json:"specificVIP,omitempty"`
}
```

### HTTPLoadBalancer (30+ fields)

Same advertise types as TCPLoadBalancer (shared). HTTP-LB-specific:

```go
type HTTPConfig struct {
    DNSVolterraManaged bool   `json:"dnsVolterraManaged,omitempty"`
    Port               uint32 `json:"port,omitempty"`
}

type HTTPSConfig struct {
    HTTPRedirect     bool              `json:"httpRedirect,omitempty"`
    AddHSTS          bool              `json:"addHSTS,omitempty"`
    TLSCertificates  []TLSCertificateRef `json:"tlsCertificates,omitempty"`
    DefaultSecurity  *struct{}         `json:"defaultSecurity,omitempty"`
    LowSecurity      *struct{}         `json:"lowSecurity,omitempty"`
    MediumSecurity   *struct{}         `json:"mediumSecurity,omitempty"`
    CustomSecurity   *CustomTLSSecurity `json:"customSecurity,omitempty"`
    NoMTLS           *struct{}         `json:"noMTLS,omitempty"`
    UseMTLS          *UseMTLS          `json:"useMTLS,omitempty"`
    Port             uint32            `json:"port,omitempty"`
}

type HTTPSAutoCertConfig struct {
    HTTPRedirect bool      `json:"httpRedirect,omitempty"`
    AddHSTS      bool      `json:"addHSTS,omitempty"`
    NoMTLS       *struct{} `json:"noMTLS,omitempty"`
    UseMTLS      *UseMTLS  `json:"useMTLS,omitempty"`
    Port         uint32    `json:"port,omitempty"`
}

type BotDefenseConfig struct {
    RegionalEndpoint string               `json:"regionalEndpoint,omitempty"`
    Policy           *BotDefensePolicy    `json:"policy,omitempty"`
    Timeout          uint32               `json:"timeout,omitempty"`
}

type BotDefensePolicy struct {
    ProtectedAppEndpoints []ProtectedAppEndpoint `json:"protectedAppEndpoints,omitempty"`
    JSInsertionRules      *JSInsertionRules      `json:"jsInsertionRules,omitempty"`
    JsDownloadPath        string                 `json:"jsDownloadPath,omitempty"`
    DisableMobileSDK      *struct{}              `json:"disableMobileSDK,omitempty"`
}

type ProtectedAppEndpoint struct {
    Metadata    map[string]string `json:"metadata,omitempty"`
    HTTPMethods []string          `json:"httpMethods,omitempty"`
    Path        PathMatcher       `json:"path"`
    Flow        string            `json:"flow,omitempty"`
    Protocol    string            `json:"protocol,omitempty"`
    AnyDomain   *struct{}         `json:"anyDomain,omitempty"`
    Domain      string            `json:"domain,omitempty"`
    MitigationAction map[string]interface{} `json:"mitigationAction,omitempty"`
}

type EnableAPIDiscoveryConfig struct {
    EnableLearnFromRedirectTraffic  *struct{} `json:"enableLearnFromRedirectTraffic,omitempty"`
    DisableLearnFromRedirectTraffic *struct{} `json:"disableLearnFromRedirectTraffic,omitempty"`
    DefaultAPIAuthDiscovery         *struct{} `json:"defaultAPIAuthDiscovery,omitempty"`
    APICrawler                      *ObjectRef `json:"apiCrawler,omitempty"`
    APIDiscoveryFromCodeScan        *ObjectRef `json:"apiDiscoveryFromCodeScan,omitempty"`
    DiscoveredAPISettings           *DiscoveredAPISettings `json:"discoveredAPISettings,omitempty"`
    SensitiveDataDetectionRules     *SensitiveDataDetectionRules `json:"sensitiveDataDetectionRules,omitempty"`
}

type DiscoveredAPISettings struct {
    PurgeByDurationDays uint32 `json:"purgeByDurationDays,omitempty"`
}

type SensitiveDataDetectionRules struct {
    SensitiveDataTypes []SensitiveDataType `json:"sensitiveDataTypes,omitempty"`
}

type SensitiveDataType struct {
    Type string `json:"type"`
}

type EnableIPReputationConfig struct {
    IPThreatCategories []string `json:"ipThreatCategories,omitempty"`
}

type RateLimitConfig struct {
    RateLimiter    *RateLimiterInline `json:"rateLimiter,omitempty"`
    // IP allowed list OneOf
    NoIPAllowedList *struct{} `json:"noIPAllowedList,omitempty"`
    IPAllowedList   []ObjectRef `json:"ipAllowedList,omitempty"`
    // Policies OneOf
    NoPolicies *struct{} `json:"noPolicies,omitempty"`
    Policies   *RateLimitPolicies `json:"policies,omitempty"`
}

type RateLimiterInline struct {
    TotalNumber     uint32 `json:"totalNumber"`
    Unit            string `json:"unit"`
    BurstMultiplier uint32 `json:"burstMultiplier"`
}

type RateLimitPolicies struct {
    Policies []ObjectRef `json:"policies,omitempty"`
}

type JSChallengeConfig struct {
    JSScriptDelay uint32 `json:"jsScriptDelay,omitempty"`
    CookieExpiry  uint32 `json:"cookieExpiry,omitempty"`
    CustomPage    string `json:"customPage,omitempty"`
}

type CaptchaChallengeConfig struct {
    Expiry     uint32 `json:"expiry,omitempty"`
    CustomPage string `json:"customPage,omitempty"`
}

type PolicyBasedChallengeConfig struct {
    DefaultJSChallengeParameters      *JSChallengeConfig      `json:"defaultJSChallengeParameters,omitempty"`
    DefaultCaptchaChallengeParameters *CaptchaChallengeConfig `json:"defaultCaptchaChallengeParameters,omitempty"`
    DefaultMitigationSettings         *struct{}               `json:"defaultMitigationSettings,omitempty"`
    AlwaysEnableJSChallenge           *struct{}               `json:"alwaysEnableJSChallenge,omitempty"`
    AlwaysEnableCaptcha               *struct{}               `json:"alwaysEnableCaptcha,omitempty"`
    NoChallenge                       *struct{}               `json:"noChallenge,omitempty"`
    MaliciousUserMitigationBypass     *struct{}               `json:"maliciousUserMitigationBypass,omitempty"`
    RuleList                          *ChallengeRuleList      `json:"ruleList,omitempty"`
    TemporaryBlockingParameters       *TemporaryBlockingParams `json:"temporaryBlockingParameters,omitempty"`
}

type ChallengeRuleList struct {
    Rules []ChallengeRule `json:"rules,omitempty"`
}

type ChallengeRule struct {
    Metadata map[string]string `json:"metadata,omitempty"`
    Spec     *ChallengeRuleSpec `json:"spec,omitempty"`
}

type ChallengeRuleSpec struct {
    ChallengeAction string `json:"challengeAction,omitempty"`
}

type TemporaryBlockingParams struct {
    Duration uint32 `json:"duration,omitempty"`
}

type CookieStickinessConfig struct {
    Name string `json:"name,omitempty"`
    Path string `json:"path,omitempty"`
    TTL  uint32 `json:"ttl,omitempty"`
}

type RingHashConfig struct {
    HashPolicy []HashPolicy `json:"hashPolicy,omitempty"`
}

type HashPolicy struct {
    HeaderName  string    `json:"headerName,omitempty"`
    CookieName  *CookieForHashing `json:"cookie,omitempty"`
    SourceIP    *struct{} `json:"sourceIP,omitempty"`
    Terminal    bool      `json:"terminal,omitempty"`
}

type CookieForHashing struct {
    Name string `json:"name"`
    TTL  uint32 `json:"ttl,omitempty"`
    Path string `json:"path,omitempty"`
}

type ActiveServicePoliciesConfig struct {
    Policies []ObjectRef `json:"policies"`
}

// Shared matcher types
type PathMatcher struct {
    Prefix string `json:"prefix,omitempty"`
    Path   string `json:"path,omitempty"`
    Regex  string `json:"regex,omitempty"`
}

type HeaderMatcher struct {
    Name   string `json:"name"`
    Exact  string `json:"exact,omitempty"`
    Regex  string `json:"regex,omitempty"`
    Presence bool `json:"presence,omitempty"`
    InvertMatch bool `json:"invertMatch,omitempty"`
}

type HTTPMethodMatcher struct {
    Methods    []string  `json:"methods,omitempty"`
    InvertMatcher bool   `json:"invertMatcher,omitempty"`
}

type IPMatcher struct {
    Prefixes    []string `json:"prefixes,omitempty"`
    InvertMatch bool     `json:"invertMatch,omitempty"`
}

type ASNMatcher struct {
    ASNumbers []uint32 `json:"asNumbers,omitempty"`
}

type LabelSelector struct {
    Expressions []string `json:"expressions"`
}

type JSInsertionRules struct {
    Rules []JSInsertionRule `json:"rules,omitempty"`
}

type JSInsertionRule struct {
    ExcludedPaths []PathMatcher `json:"excludedPaths,omitempty"`
}
```

## Implementation approach

1. Define all new types in `api/v1alpha1/` — shared types in `shared_types.go`, CRD-specific types in respective `*_types.go` files
2. Update each CRD spec to use the new types instead of `*apiextensionsv1.JSON`
3. Update xcclient types to use `json.RawMessage` consistently (no change needed — the mapper layer handles translation)
4. Update controller mappers to convert between CRD types (camelCase) and xcclient types (snake_case via json.RawMessage)
5. Run `make generate && make manifests` to regenerate CRD YAML and deepcopy
6. Update existing tests (contract tests, mapper tests, integration tests)
7. Validate with contract tests against real XC tenant

## What changes per CRD

For each CRD, 3-4 files change:
- `api/v1alpha1/*_types.go` — replace `*apiextensionsv1.JSON` with typed structs
- `internal/controller/*_mapper.go` — update mapper to convert typed structs to xcclient format
- `internal/xcclient/*.go` — xcclient types stay as `json.RawMessage` (no change in most cases)
- Tests — update to use typed structs instead of `json.RawMessage`

## Testing

- Existing contract tests must continue to pass (validates real XC API compatibility)
- Existing integration tests must continue to pass
- Mapper tests updated to use typed structs
- `make test` green before merge
