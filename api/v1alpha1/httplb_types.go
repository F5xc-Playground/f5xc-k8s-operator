package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hlb
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HTTPLoadBalancer is the Schema for the httploadbalancers API.
type HTTPLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HTTPLoadBalancerSpec   `json:"spec,omitempty"`
	Status HTTPLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HTTPLoadBalancerList contains a list of HTTPLoadBalancer.
type HTTPLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPLoadBalancer `json:"items"`
}

// HTTPLoadBalancerSpec defines the desired state of an HTTPLoadBalancer.
type HTTPLoadBalancerSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace       string                 `json:"xcNamespace"`
	Domains           []string               `json:"domains"`
	DefaultRoutePools []RoutePool            `json:"defaultRoutePools"`
	Routes            []apiextensionsv1.JSON `json:"routes,omitempty"`

	// TLS OneOf
	HTTP          *HTTPConfig          `json:"http,omitempty"`
	HTTPS         *HTTPSConfig         `json:"https,omitempty"`
	HTTPSAutoCert *HTTPSAutoCertConfig `json:"httpsAutoCert,omitempty"`

	// WAF OneOf
	DisableWAF  *EmptyObject `json:"disableWAF,omitempty"`
	AppFirewall *ObjectRef   `json:"appFirewall,omitempty"`

	// Bot defense OneOf
	DisableBotDefense *EmptyObject      `json:"disableBotDefense,omitempty"`
	BotDefense        *BotDefenseConfig `json:"botDefense,omitempty"`

	// API discovery OneOf
	DisableAPIDiscovery *EmptyObject              `json:"disableAPIDiscovery,omitempty"`
	EnableAPIDiscovery  *EnableAPIDiscoveryConfig `json:"enableAPIDiscovery,omitempty"`

	// IP reputation OneOf
	DisableIPReputation *EmptyObject              `json:"disableIPReputation,omitempty"`
	EnableIPReputation  *EnableIPReputationConfig `json:"enableIPReputation,omitempty"`

	// Rate limit OneOf
	DisableRateLimit *EmptyObject     `json:"disableRateLimit,omitempty"`
	RateLimit        *RateLimitConfig `json:"rateLimit,omitempty"`

	// Challenge OneOf
	NoChallenge          *EmptyObject                `json:"noChallenge,omitempty"`
	JSChallenge          *JSChallengeConfig          `json:"jsChallenge,omitempty"`
	CaptchaChallenge     *CaptchaChallengeConfig     `json:"captchaChallenge,omitempty"`
	PolicyBasedChallenge *PolicyBasedChallengeConfig `json:"policyBasedChallenge,omitempty"`

	// LB algorithm OneOf
	RoundRobin         *EmptyObject            `json:"roundRobin,omitempty"`
	LeastActive        *EmptyObject            `json:"leastActive,omitempty"`
	Random             *EmptyObject            `json:"random,omitempty"`
	SourceIPStickiness *EmptyObject            `json:"sourceIPStickiness,omitempty"`
	CookieStickiness   *CookieStickinessConfig `json:"cookieStickiness,omitempty"`
	RingHash           *RingHashConfig         `json:"ringHash,omitempty"`

	// Advertise OneOf
	AdvertiseOnPublicDefaultVIP *EmptyObject       `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *AdvertiseOnPublic `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *AdvertiseCustom   `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *EmptyObject       `json:"doNotAdvertise,omitempty"`

	// Service policies OneOf
	ServicePoliciesFromNamespace *EmptyObject                 `json:"servicePoliciesFromNamespace,omitempty"`
	ActiveServicePolicies        *ActiveServicePoliciesConfig `json:"activeServicePolicies,omitempty"`
	NoServicePolicies            *EmptyObject                 `json:"noServicePolicies,omitempty"`

	// User ID OneOf
	UserIDClientIP     *EmptyObject `json:"userIDClientIP,omitempty"`
	UserIdentification *ObjectRef   `json:"userIdentification,omitempty"`

	// API definition OneOf
	DisableAPIDefinition *EmptyObject           `json:"disableAPIDefinition,omitempty"`
	APISpecification     *APISpecificationConfig `json:"apiSpecification,omitempty"`

	// Malicious user detection OneOf
	DisableMaliciousUserDetection *EmptyObject `json:"disableMaliciousUserDetection,omitempty"`
	EnableMaliciousUserDetection  *EmptyObject `json:"enableMaliciousUserDetection,omitempty"`
}

// HTTPLoadBalancerStatus defines the observed state of HTTPLoadBalancer.
type HTTPLoadBalancerStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

// HTTPConfig configures plain HTTP on the load balancer.
type HTTPConfig struct {
	DNSVolterraManaged bool   `json:"dnsVolterraManaged,omitempty"`
	Port               uint32 `json:"port,omitempty"`
}

// HTTPSConfig configures HTTPS with explicit certificates.
type HTTPSConfig struct {
	HTTPRedirect    bool                `json:"httpRedirect,omitempty"`
	AddHSTS         bool                `json:"addHSTS,omitempty"`
	TLSCertificates []TLSCertificateRef `json:"tlsCertificates,omitempty"`
	DefaultSecurity *EmptyObject        `json:"defaultSecurity,omitempty"`
	LowSecurity     *EmptyObject        `json:"lowSecurity,omitempty"`
	MediumSecurity  *EmptyObject        `json:"mediumSecurity,omitempty"`
	CustomSecurity  *CustomTLSSecurity  `json:"customSecurity,omitempty"`
	NoMTLS          *EmptyObject        `json:"noMTLS,omitempty"`
	UseMTLS         *UseMTLS            `json:"useMTLS,omitempty"`
	Port            uint32              `json:"port,omitempty"`
}

// HTTPSAutoCertConfig configures HTTPS with auto-managed certificates.
type HTTPSAutoCertConfig struct {
	HTTPRedirect bool         `json:"httpRedirect,omitempty"`
	AddHSTS      bool         `json:"addHSTS,omitempty"`
	NoMTLS       *EmptyObject `json:"noMTLS,omitempty"`
	UseMTLS      *UseMTLS     `json:"useMTLS,omitempty"`
	Port         uint32       `json:"port,omitempty"`
}

// BotDefenseConfig configures bot defense.
type BotDefenseConfig struct {
	RegionalEndpoint string            `json:"regionalEndpoint,omitempty"`
	Policy           *BotDefensePolicy `json:"policy,omitempty"`
	Timeout          uint32            `json:"timeout,omitempty"`
}

// BotDefensePolicy holds the bot defense policy.
type BotDefensePolicy struct {
	ProtectedAppEndpoints []ProtectedAppEndpoint `json:"protectedAppEndpoints,omitempty"`
	JSInsertionRules      *JSInsertionRules      `json:"jsInsertionRules,omitempty"`
	JsDownloadPath        string                 `json:"jsDownloadPath,omitempty"`
	DisableMobileSDK      *EmptyObject           `json:"disableMobileSDK,omitempty"`
}

// ProtectedAppEndpoint defines a protected application endpoint for bot defense.
type ProtectedAppEndpoint struct {
	Metadata         map[string]string `json:"metadata,omitempty"`
	HTTPMethods      []string          `json:"httpMethods,omitempty"`
	Path             PathMatcher       `json:"path"`
	Flow             string            `json:"flow,omitempty"`
	Protocol         string            `json:"protocol,omitempty"`
	AnyDomain        *EmptyObject      `json:"anyDomain,omitempty"`
	Domain           string            `json:"domain,omitempty"`
	MitigationAction *apiextensionsv1.JSON `json:"mitigationAction,omitempty"`
}

// JSInsertionRules holds JS insertion rules for bot defense.
type JSInsertionRules struct {
	Rules []JSInsertionRule `json:"rules,omitempty"`
}

// JSInsertionRule defines a single JS insertion rule.
type JSInsertionRule struct {
	ExcludedPaths []PathMatcher `json:"excludedPaths,omitempty"`
}

// EnableAPIDiscoveryConfig configures API discovery.
type EnableAPIDiscoveryConfig struct {
	EnableLearnFromRedirectTraffic  *EmptyObject                 `json:"enableLearnFromRedirectTraffic,omitempty"`
	DisableLearnFromRedirectTraffic *EmptyObject                 `json:"disableLearnFromRedirectTraffic,omitempty"`
	DefaultAPIAuthDiscovery         *EmptyObject                 `json:"defaultAPIAuthDiscovery,omitempty"`
	APICrawler                      *ObjectRef                   `json:"apiCrawler,omitempty"`
	APIDiscoveryFromCodeScan        *ObjectRef                   `json:"apiDiscoveryFromCodeScan,omitempty"`
	DiscoveredAPISettings           *DiscoveredAPISettings       `json:"discoveredAPISettings,omitempty"`
	SensitiveDataDetectionRules     *SensitiveDataDetectionRules `json:"sensitiveDataDetectionRules,omitempty"`
}

// DiscoveredAPISettings configures discovered API settings.
type DiscoveredAPISettings struct {
	PurgeByDurationDays uint32 `json:"purgeByDurationDays,omitempty"`
}

// SensitiveDataDetectionRules holds sensitive data detection rules.
type SensitiveDataDetectionRules struct {
	SensitiveDataTypes []SensitiveDataType `json:"sensitiveDataTypes,omitempty"`
}

// SensitiveDataType identifies a sensitive data type.
type SensitiveDataType struct {
	Type string `json:"type"`
}

// EnableIPReputationConfig configures IP reputation filtering.
type EnableIPReputationConfig struct {
	IPThreatCategories []string `json:"ipThreatCategories,omitempty"`
}

// RateLimitConfig configures rate limiting.
type RateLimitConfig struct {
	RateLimiter     *RateLimiterInline `json:"rateLimiter,omitempty"`
	NoIPAllowedList *EmptyObject       `json:"noIPAllowedList,omitempty"`
	IPAllowedList   []ObjectRef        `json:"ipAllowedList,omitempty"`
	NoPolicies      *EmptyObject       `json:"noPolicies,omitempty"`
	Policies        *RateLimitPolicies `json:"policies,omitempty"`
}

// RateLimiterInline defines an inline rate limiter.
type RateLimiterInline struct {
	TotalNumber     uint32 `json:"totalNumber"`
	Unit            string `json:"unit"`
	BurstMultiplier uint32 `json:"burstMultiplier"`
}

// RateLimitPolicies holds a list of rate limit policy references.
type RateLimitPolicies struct {
	Policies []ObjectRef `json:"policies,omitempty"`
}

// JSChallengeConfig configures a JavaScript challenge.
type JSChallengeConfig struct {
	JSScriptDelay uint32 `json:"jsScriptDelay,omitempty"`
	CookieExpiry  uint32 `json:"cookieExpiry,omitempty"`
	CustomPage    string `json:"customPage,omitempty"`
}

// CaptchaChallengeConfig configures a CAPTCHA challenge.
type CaptchaChallengeConfig struct {
	Expiry     uint32 `json:"expiry,omitempty"`
	CustomPage string `json:"customPage,omitempty"`
}

// PolicyBasedChallengeConfig configures policy-based challenges.
type PolicyBasedChallengeConfig struct {
	DefaultJSChallengeParameters      *JSChallengeConfig      `json:"defaultJSChallengeParameters,omitempty"`
	DefaultCaptchaChallengeParameters *CaptchaChallengeConfig `json:"defaultCaptchaChallengeParameters,omitempty"`
	DefaultMitigationSettings        *EmptyObject            `json:"defaultMitigationSettings,omitempty"`
	AlwaysEnableJSChallenge          *EmptyObject            `json:"alwaysEnableJSChallenge,omitempty"`
	AlwaysEnableCaptcha              *EmptyObject            `json:"alwaysEnableCaptcha,omitempty"`
	NoChallenge                      *EmptyObject            `json:"noChallenge,omitempty"`
	MaliciousUserMitigationBypass    *EmptyObject            `json:"maliciousUserMitigationBypass,omitempty"`
	MaliciousUserMitigation          *ObjectRef              `json:"maliciousUserMitigation,omitempty"`
	RuleList                         *ChallengeRuleList      `json:"ruleList,omitempty"`
	TemporaryBlockingParameters      *TemporaryBlockingParams `json:"temporaryBlockingParameters,omitempty"`
}

// ChallengeRuleList holds a list of challenge rules.
type ChallengeRuleList struct {
	Rules []ChallengeRule `json:"rules,omitempty"`
}

// ChallengeRule defines a single challenge rule.
type ChallengeRule struct {
	Metadata map[string]string  `json:"metadata,omitempty"`
	Spec     *ChallengeRuleSpec `json:"spec,omitempty"`
}

// ChallengeRuleSpec defines the spec of a challenge rule.
type ChallengeRuleSpec struct {
	ChallengeAction string `json:"challengeAction,omitempty"`
}

// TemporaryBlockingParams configures temporary blocking parameters.
type TemporaryBlockingParams struct {
	Duration uint32 `json:"duration,omitempty"`
}

// CookieStickinessConfig configures cookie-based sticky sessions.
type CookieStickinessConfig struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	TTL  uint32 `json:"ttl,omitempty"`
}

// RingHashConfig configures ring-hash load balancing.
type RingHashConfig struct {
	HashPolicy []HashPolicy `json:"hashPolicy,omitempty"`
}

// HashPolicy defines a single hash policy entry for ring-hash LB.
type HashPolicy struct {
	HeaderName string            `json:"headerName,omitempty"`
	Cookie     *CookieForHashing `json:"cookie,omitempty"`
	SourceIP   *EmptyObject      `json:"sourceIP,omitempty"`
	Terminal   bool              `json:"terminal,omitempty"`
}

// CookieForHashing identifies a cookie used for ring-hash load balancing.
type CookieForHashing struct {
	Name string `json:"name"`
	TTL  uint32 `json:"ttl,omitempty"`
	Path string `json:"path,omitempty"`
}

// ActiveServicePoliciesConfig holds the active service policies list.
type ActiveServicePoliciesConfig struct {
	Policies []ObjectRef `json:"policies"`
}

// APISpecificationConfig wraps an APIDefinition reference with validation settings.
type APISpecificationConfig struct {
	APIDefinition              *ObjectRef            `json:"apiDefinition"`
	ValidationDisabled         *EmptyObject          `json:"validationDisabled,omitempty"`
	ValidationAllSpecEndpoints *EmptyObject          `json:"validationAllSpecEndpoints,omitempty"`
	ValidationCustomList       *ValidationCustomList `json:"validationCustomList,omitempty"`
}

// ValidationCustomList holds a custom list of endpoint validations.
type ValidationCustomList struct {
	EndpointValidationList []APIOperation `json:"endpointValidationList,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HTTPLoadBalancer{}, &HTTPLoadBalancerList{})
}
