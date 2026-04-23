package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func buildHTTPLoadBalancerCreate(cr *v1alpha1.HTTPLoadBalancer, xcNamespace string) *xcclient.HTTPLoadBalancerCreate {
	return &xcclient.HTTPLoadBalancerCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapHTTPLoadBalancerSpec(&cr.Spec),
	}
}

func buildHTTPLoadBalancerReplace(cr *v1alpha1.HTTPLoadBalancer, xcNamespace, resourceVersion string) *xcclient.HTTPLoadBalancerReplace {
	return &xcclient.HTTPLoadBalancerReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapHTTPLoadBalancerSpec(&cr.Spec),
	}
}

func buildHTTPLoadBalancerDesiredSpecJSON(cr *v1alpha1.HTTPLoadBalancer, xcNamespace string) (json.RawMessage, error) {
	create := buildHTTPLoadBalancerCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

// ---------------------------------------------------------------------------
// Wire-format types (snake_case JSON tags — these mirror the F5XC REST API)
// ---------------------------------------------------------------------------

type xcHTTPConfig struct {
	DNSVolterraManaged bool   `json:"dns_volterra_managed,omitempty"`
	Port               uint32 `json:"port,omitempty"`
}

type xcHTTPSConfig struct {
	HTTPRedirect    bool                  `json:"http_redirect,omitempty"`
	AddHSTS         bool                  `json:"add_hsts,omitempty"`
	TLSCertificates []xcTLSCertificateRef `json:"tls_certificates,omitempty"`
	DefaultSecurity *v1alpha1.EmptyObject `json:"default_security,omitempty"`
	LowSecurity     *v1alpha1.EmptyObject `json:"low_security,omitempty"`
	MediumSecurity  *v1alpha1.EmptyObject `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity  `json:"custom_security,omitempty"`
	NoMTLS          *v1alpha1.EmptyObject `json:"no_mtls,omitempty"`
	UseMTLS         *xcUseMTLS            `json:"use_mtls,omitempty"`
	Port            uint32                `json:"port,omitempty"`
}

type xcHTTPSAutoCertConfig struct {
	HTTPRedirect bool                  `json:"http_redirect,omitempty"`
	AddHSTS      bool                  `json:"add_hsts,omitempty"`
	NoMTLS       *v1alpha1.EmptyObject `json:"no_mtls,omitempty"`
	UseMTLS      *xcUseMTLS            `json:"use_mtls,omitempty"`
	Port         uint32                `json:"port,omitempty"`
}

type xcBotDefenseConfig struct {
	RegionalEndpoint string              `json:"regional_endpoint,omitempty"`
	Policy           *xcBotDefensePolicy `json:"policy,omitempty"`
	Timeout          uint32              `json:"timeout,omitempty"`
}

type xcBotDefensePolicy struct {
	ProtectedAppEndpoints []xcProtectedAppEndpoint `json:"protected_app_endpoints,omitempty"`
	JSInsertionRules      *xcJSInsertionRules      `json:"js_insertion_rules,omitempty"`
	JsDownloadPath        string                   `json:"js_download_path,omitempty"`
	DisableMobileSDK      *v1alpha1.EmptyObject    `json:"disable_mobile_sdk,omitempty"`
}

type xcProtectedAppEndpoint struct {
	Metadata         map[string]string     `json:"metadata,omitempty"`
	HTTPMethods      []string              `json:"http_methods,omitempty"`
	Path             xcPathMatcher         `json:"path"`
	Flow             string                `json:"flow,omitempty"`
	Protocol         string                `json:"protocol,omitempty"`
	AnyDomain        *v1alpha1.EmptyObject `json:"any_domain,omitempty"`
	Domain           string                `json:"domain,omitempty"`
	MitigationAction *apiextensionsv1.JSON `json:"mitigation_action,omitempty"`
}

type xcJSInsertionRules struct {
	Rules []xcJSInsertionRule `json:"rules,omitempty"`
}

type xcJSInsertionRule struct {
	ExcludedPaths []xcPathMatcher `json:"excluded_paths,omitempty"`
}

type xcEnableAPIDiscoveryConfig struct {
	EnableLearnFromRedirectTraffic  *v1alpha1.EmptyObject          `json:"enable_learn_from_redirect_traffic,omitempty"`
	DisableLearnFromRedirectTraffic *v1alpha1.EmptyObject          `json:"disable_learn_from_redirect_traffic,omitempty"`
	DefaultAPIAuthDiscovery         *v1alpha1.EmptyObject          `json:"default_api_auth_discovery,omitempty"`
	APICrawler                      *xcObjectRef                   `json:"api_crawler,omitempty"`
	APIDiscoveryFromCodeScan        *xcObjectRef                   `json:"api_discovery_from_code_scan,omitempty"`
	DiscoveredAPISettings           *xcDiscoveredAPISettings       `json:"discovered_api_settings,omitempty"`
	SensitiveDataDetectionRules     *xcSensitiveDataDetectionRules `json:"sensitive_data_detection_rules,omitempty"`
}

type xcDiscoveredAPISettings struct {
	PurgeByDurationDays uint32 `json:"purge_duration_days,omitempty"`
}

type xcSensitiveDataDetectionRules struct {
	SensitiveDataTypes []xcSensitiveDataType `json:"sensitive_data_types,omitempty"`
}

type xcSensitiveDataType struct {
	Type string `json:"type"`
}

type xcEnableIPReputationConfig struct {
	IPThreatCategories []string `json:"ip_threat_categories,omitempty"`
}

type xcRateLimitConfig struct {
	RateLimiter     *xcRateLimiterInline  `json:"rate_limiter,omitempty"`
	NoIPAllowedList *v1alpha1.EmptyObject `json:"no_ip_allowed_list,omitempty"`
	IPAllowedList   []xcObjectRef         `json:"ip_allowed_list,omitempty"`
	NoPolicies      *v1alpha1.EmptyObject `json:"no_policies,omitempty"`
	Policies        *xcRateLimitPolicies  `json:"policies,omitempty"`
}

type xcRateLimiterInline struct {
	TotalNumber     uint32 `json:"total_number"`
	Unit            string `json:"unit"`
	BurstMultiplier uint32 `json:"burst_multiplier"`
}

type xcRateLimitPolicies struct {
	Policies []xcObjectRef `json:"policies,omitempty"`
}

type xcJSChallengeConfig struct {
	JSScriptDelay uint32 `json:"js_script_delay,omitempty"`
	CookieExpiry  uint32 `json:"cookie_expiry,omitempty"`
	CustomPage    string `json:"custom_page,omitempty"`
}

type xcCaptchaChallengeConfig struct {
	Expiry     uint32 `json:"expiry,omitempty"`
	CustomPage string `json:"custom_page,omitempty"`
}

type xcPolicyBasedChallengeConfig struct {
	DefaultJSChallengeParameters      *xcJSChallengeConfig      `json:"default_js_challenge_parameters,omitempty"`
	DefaultCaptchaChallengeParameters *xcCaptchaChallengeConfig `json:"default_captcha_challenge_parameters,omitempty"`
	DefaultMitigationSettings         *v1alpha1.EmptyObject     `json:"default_mitigation_settings,omitempty"`
	AlwaysEnableJSChallenge           *v1alpha1.EmptyObject     `json:"always_enable_js_challenge,omitempty"`
	AlwaysEnableCaptcha               *v1alpha1.EmptyObject     `json:"always_enable_captcha,omitempty"`
	NoChallenge                       *v1alpha1.EmptyObject     `json:"no_challenge,omitempty"`
	MaliciousUserMitigationBypass     *v1alpha1.EmptyObject     `json:"malicious_user_mitigation_bypass,omitempty"`
	MaliciousUserMitigation           *xcObjectRef              `json:"malicious_user_mitigation,omitempty"`
	RuleList                          *xcChallengeRuleList      `json:"rule_list,omitempty"`
	TemporaryBlockingParameters       *xcTemporaryBlockingParams `json:"temporary_blocking_parameters,omitempty"`
}

type xcChallengeRuleList struct {
	Rules []xcChallengeRule `json:"rules,omitempty"`
}

type xcChallengeRule struct {
	Metadata map[string]string    `json:"metadata,omitempty"`
	Spec     *xcChallengeRuleSpec `json:"spec,omitempty"`
}

type xcChallengeRuleSpec struct {
	ChallengeAction string `json:"challenge_action,omitempty"`
}

type xcTemporaryBlockingParams struct {
	Duration uint32 `json:"duration,omitempty"`
}

type xcCookieStickinessConfig struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	TTL  uint32 `json:"ttl,omitempty"`
}

type xcRingHashConfig struct {
	HashPolicy []xcHashPolicy `json:"hash_policy,omitempty"`
}

type xcHashPolicy struct {
	HeaderName string               `json:"header_name,omitempty"`
	Cookie     *xcCookieForHashing  `json:"cookie,omitempty"`
	SourceIP   *v1alpha1.EmptyObject `json:"source_ip,omitempty"`
	Terminal   bool                 `json:"terminal,omitempty"`
}

type xcCookieForHashing struct {
	Name string `json:"name"`
	TTL  uint32 `json:"ttl,omitempty"`
	Path string `json:"path,omitempty"`
}

type xcActiveServicePoliciesConfig struct {
	Policies []xcObjectRef `json:"policies"`
}

type xcAPISpecificationConfig struct {
	APIDefinition              *xcObjectRef            `json:"api_definition"`
	ValidationDisabled         *v1alpha1.EmptyObject   `json:"validation_disabled,omitempty"`
	ValidationAllSpecEndpoints *v1alpha1.EmptyObject   `json:"validation_all_spec_endpoints,omitempty"`
	ValidationCustomList       *xcValidationCustomList `json:"validation_custom_list,omitempty"`
}

type xcValidationCustomList struct {
	EndpointValidationList []xcAPIOperation `json:"endpoint_validation_list,omitempty"`
}

// ---------------------------------------------------------------------------
// Mapper
// ---------------------------------------------------------------------------

func mapHTTPLoadBalancerSpec(spec *v1alpha1.HTTPLoadBalancerSpec) xcclient.HTTPLoadBalancerSpec {
	var out xcclient.HTTPLoadBalancerSpec
	out.Domains = spec.Domains

	for i := range spec.DefaultRoutePools {
		out.DefaultRoutePools = append(out.DefaultRoutePools, mapRoutePool(&spec.DefaultRoutePools[i]))
	}

	if len(spec.Routes) > 0 {
		routesJSON, _ := json.Marshal(spec.Routes)
		out.Routes = routesJSON
	}

	// TLS OneOf
	if spec.HTTP != nil {
		port := spec.HTTP.Port
		if port == 0 {
			port = 80
		}
		out.HTTP = marshalJSON(xcHTTPConfig{DNSVolterraManaged: spec.HTTP.DNSVolterraManaged, Port: port})
	}
	if spec.HTTPS != nil {
		out.HTTPS = mapHTTPSConfig(spec.HTTPS)
	}
	if spec.HTTPSAutoCert != nil {
		out.HTTPSAutoCert = marshalJSON(xcHTTPSAutoCertConfig{
			HTTPRedirect: spec.HTTPSAutoCert.HTTPRedirect, AddHSTS: spec.HTTPSAutoCert.AddHSTS,
			NoMTLS: spec.HTTPSAutoCert.NoMTLS, UseMTLS: mapXCUseMTLS(spec.HTTPSAutoCert.UseMTLS), Port: spec.HTTPSAutoCert.Port,
		})
	}

	// WAF OneOf
	if spec.DisableWAF != nil {
		out.DisableWAF = emptyObjectJSON
	}
	if spec.AppFirewall != nil {
		out.AppFirewall = mapObjectRefPtr(spec.AppFirewall)
	}

	// Bot defense OneOf
	if spec.DisableBotDefense != nil {
		out.DisableBotDefense = emptyObjectJSON
	}
	if spec.BotDefense != nil {
		out.BotDefense = mapBotDefenseConfig(spec.BotDefense)
	}

	// API discovery OneOf
	if spec.DisableAPIDiscovery != nil {
		out.DisableAPIDiscovery = emptyObjectJSON
	}
	if spec.EnableAPIDiscovery != nil {
		out.EnableAPIDiscovery = mapEnableAPIDiscoveryConfig(spec.EnableAPIDiscovery)
	}

	// IP reputation OneOf
	if spec.DisableIPReputation != nil {
		out.DisableIPReputation = emptyObjectJSON
	}
	if spec.EnableIPReputation != nil {
		out.EnableIPReputation = marshalJSON(xcEnableIPReputationConfig{IPThreatCategories: spec.EnableIPReputation.IPThreatCategories})
	}

	// Rate limit OneOf
	if spec.DisableRateLimit != nil {
		out.DisableRateLimit = emptyObjectJSON
	}
	if spec.RateLimit != nil {
		out.RateLimit = mapRateLimitConfig(spec.RateLimit)
	}

	// Challenge OneOf
	if spec.NoChallenge != nil {
		out.NoChallenge = emptyObjectJSON
	}
	if spec.JSChallenge != nil {
		out.JSChallenge = marshalJSON(xcJSChallengeConfig{JSScriptDelay: spec.JSChallenge.JSScriptDelay, CookieExpiry: spec.JSChallenge.CookieExpiry, CustomPage: spec.JSChallenge.CustomPage})
	}
	if spec.CaptchaChallenge != nil {
		out.CaptchaChallenge = marshalJSON(xcCaptchaChallengeConfig{Expiry: spec.CaptchaChallenge.Expiry, CustomPage: spec.CaptchaChallenge.CustomPage})
	}
	if spec.PolicyBasedChallenge != nil {
		out.PolicyBasedChallenge = mapPolicyBasedChallengeConfig(spec.PolicyBasedChallenge)
	}

	// LB algorithm OneOf
	if spec.RoundRobin != nil {
		out.RoundRobin = emptyObjectJSON
	}
	if spec.LeastActive != nil {
		out.LeastActive = emptyObjectJSON
	}
	if spec.Random != nil {
		out.Random = emptyObjectJSON
	}
	if spec.SourceIPStickiness != nil {
		out.SourceIPStickiness = emptyObjectJSON
	}
	if spec.CookieStickiness != nil {
		out.CookieStickiness = marshalJSON(xcCookieStickinessConfig{Name: spec.CookieStickiness.Name, Path: spec.CookieStickiness.Path, TTL: spec.CookieStickiness.TTL})
	}
	if spec.RingHash != nil {
		out.RingHash = mapRingHash(spec.RingHash)
	}

	// Advertise OneOf
	if spec.AdvertiseOnPublicDefaultVIP != nil {
		out.AdvertiseOnPublicDefaultVIP = emptyObjectJSON
	}
	if spec.AdvertiseOnPublic != nil {
		out.AdvertiseOnPublic = mapXCAdvertiseOnPublic(spec.AdvertiseOnPublic)
	}
	if spec.AdvertiseCustom != nil {
		out.AdvertiseCustom = mapXCAdvertiseCustom(spec.AdvertiseCustom)
	}
	if spec.DoNotAdvertise != nil {
		out.DoNotAdvertise = emptyObjectJSON
	}

	// Service policies OneOf
	if spec.ServicePoliciesFromNamespace != nil {
		out.ServicePoliciesFromNamespace = emptyObjectJSON
	}
	if spec.ActiveServicePolicies != nil {
		out.ActiveServicePolicies = marshalJSON(xcActiveServicePoliciesConfig{Policies: mapXCObjectRefs(spec.ActiveServicePolicies.Policies)})
	}
	if spec.NoServicePolicies != nil {
		out.NoServicePolicies = emptyObjectJSON
	}

	// User ID OneOf
	if spec.UserIDClientIP != nil {
		out.UserIDClientIP = emptyObjectJSON
	}
	if spec.UserIdentification != nil {
		out.UserIdentification = mapObjectRefPtr(spec.UserIdentification)
	}

	// API definition OneOf
	if spec.DisableAPIDefinition != nil {
		out.DisableAPIDefinition = emptyObjectJSON
	}
	if spec.APISpecification != nil {
		out.APISpecification = mapAPISpecificationConfig(spec.APISpecification)
	}

	// Malicious user detection OneOf
	if spec.DisableMaliciousUserDetection != nil {
		out.DisableMaliciousUserDetection = emptyObjectJSON
	}
	if spec.EnableMaliciousUserDetection != nil {
		out.EnableMaliciousUserDetection = emptyObjectJSON
	}

	return out
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func mapHTTPSConfig(h *v1alpha1.HTTPSConfig) json.RawMessage {
	return marshalJSON(xcHTTPSConfig{
		HTTPRedirect:    h.HTTPRedirect,
		AddHSTS:         h.AddHSTS,
		TLSCertificates: mapXCTLSCertificateRefs(h.TLSCertificates),
		DefaultSecurity: h.DefaultSecurity,
		LowSecurity:     h.LowSecurity,
		MediumSecurity:  h.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(h.CustomSecurity),
		NoMTLS:          h.NoMTLS,
		UseMTLS:         mapXCUseMTLS(h.UseMTLS),
		Port:            h.Port,
	})
}

func mapBotDefenseConfig(bd *v1alpha1.BotDefenseConfig) json.RawMessage {
	wire := xcBotDefenseConfig{RegionalEndpoint: bd.RegionalEndpoint, Timeout: bd.Timeout}
	if bd.Policy != nil {
		p := &xcBotDefensePolicy{JsDownloadPath: bd.Policy.JsDownloadPath, DisableMobileSDK: bd.Policy.DisableMobileSDK}
		for _, ep := range bd.Policy.ProtectedAppEndpoints {
			p.ProtectedAppEndpoints = append(p.ProtectedAppEndpoints, xcProtectedAppEndpoint{
				Metadata:         ep.Metadata,
				HTTPMethods:      ep.HTTPMethods,
				Path:             mapXCPathMatcher(ep.Path),
				Flow:             ep.Flow,
				Protocol:         ep.Protocol,
				AnyDomain:        ep.AnyDomain,
				Domain:           ep.Domain,
				MitigationAction: ep.MitigationAction,
			})
		}
		if bd.Policy.JSInsertionRules != nil {
			jsr := &xcJSInsertionRules{}
			for _, r := range bd.Policy.JSInsertionRules.Rules {
				xr := xcJSInsertionRule{}
				for _, ep := range r.ExcludedPaths {
					xr.ExcludedPaths = append(xr.ExcludedPaths, mapXCPathMatcher(ep))
				}
				jsr.Rules = append(jsr.Rules, xr)
			}
			p.JSInsertionRules = jsr
		}
		wire.Policy = p
	}
	return marshalJSON(wire)
}

func mapEnableAPIDiscoveryConfig(ad *v1alpha1.EnableAPIDiscoveryConfig) json.RawMessage {
	wire := xcEnableAPIDiscoveryConfig{
		EnableLearnFromRedirectTraffic:  ad.EnableLearnFromRedirectTraffic,
		DisableLearnFromRedirectTraffic: ad.DisableLearnFromRedirectTraffic,
		DefaultAPIAuthDiscovery:         ad.DefaultAPIAuthDiscovery,
		APICrawler:                      mapXCObjectRef(ad.APICrawler),
		APIDiscoveryFromCodeScan:        mapXCObjectRef(ad.APIDiscoveryFromCodeScan),
	}
	if ad.DiscoveredAPISettings != nil {
		wire.DiscoveredAPISettings = &xcDiscoveredAPISettings{PurgeByDurationDays: ad.DiscoveredAPISettings.PurgeByDurationDays}
	}
	if ad.SensitiveDataDetectionRules != nil {
		sdr := &xcSensitiveDataDetectionRules{}
		for _, sdt := range ad.SensitiveDataDetectionRules.SensitiveDataTypes {
			sdr.SensitiveDataTypes = append(sdr.SensitiveDataTypes, xcSensitiveDataType{Type: sdt.Type})
		}
		wire.SensitiveDataDetectionRules = sdr
	}
	return marshalJSON(wire)
}

func mapRateLimitConfig(rl *v1alpha1.RateLimitConfig) json.RawMessage {
	wire := xcRateLimitConfig{NoIPAllowedList: rl.NoIPAllowedList, IPAllowedList: mapXCObjectRefs(rl.IPAllowedList), NoPolicies: rl.NoPolicies}
	if rl.RateLimiter != nil {
		wire.RateLimiter = &xcRateLimiterInline{TotalNumber: rl.RateLimiter.TotalNumber, Unit: rl.RateLimiter.Unit, BurstMultiplier: rl.RateLimiter.BurstMultiplier}
	}
	if rl.Policies != nil {
		wire.Policies = &xcRateLimitPolicies{Policies: mapXCObjectRefs(rl.Policies.Policies)}
	}
	return marshalJSON(wire)
}

func mapPolicyBasedChallengeConfig(pbc *v1alpha1.PolicyBasedChallengeConfig) json.RawMessage {
	wire := xcPolicyBasedChallengeConfig{
		DefaultMitigationSettings:     pbc.DefaultMitigationSettings,
		AlwaysEnableJSChallenge:       pbc.AlwaysEnableJSChallenge,
		AlwaysEnableCaptcha:           pbc.AlwaysEnableCaptcha,
		NoChallenge:                   pbc.NoChallenge,
		MaliciousUserMitigationBypass: pbc.MaliciousUserMitigationBypass,
		MaliciousUserMitigation:       mapXCObjectRef(pbc.MaliciousUserMitigation),
	}
	if pbc.DefaultJSChallengeParameters != nil {
		wire.DefaultJSChallengeParameters = &xcJSChallengeConfig{
			JSScriptDelay: pbc.DefaultJSChallengeParameters.JSScriptDelay,
			CookieExpiry:  pbc.DefaultJSChallengeParameters.CookieExpiry,
			CustomPage:    pbc.DefaultJSChallengeParameters.CustomPage,
		}
	}
	if pbc.DefaultCaptchaChallengeParameters != nil {
		wire.DefaultCaptchaChallengeParameters = &xcCaptchaChallengeConfig{
			Expiry:     pbc.DefaultCaptchaChallengeParameters.Expiry,
			CustomPage: pbc.DefaultCaptchaChallengeParameters.CustomPage,
		}
	}
	if pbc.RuleList != nil {
		rl := &xcChallengeRuleList{}
		for _, r := range pbc.RuleList.Rules {
			xr := xcChallengeRule{Metadata: r.Metadata}
			if r.Spec != nil {
				xr.Spec = &xcChallengeRuleSpec{ChallengeAction: r.Spec.ChallengeAction}
			}
			rl.Rules = append(rl.Rules, xr)
		}
		wire.RuleList = rl
	}
	if pbc.TemporaryBlockingParameters != nil {
		wire.TemporaryBlockingParameters = &xcTemporaryBlockingParams{Duration: pbc.TemporaryBlockingParameters.Duration}
	}
	return marshalJSON(wire)
}

func mapAPISpecificationConfig(as *v1alpha1.APISpecificationConfig) json.RawMessage {
	wire := xcAPISpecificationConfig{
		APIDefinition:              mapXCObjectRef(as.APIDefinition),
		ValidationDisabled:         as.ValidationDisabled,
		ValidationAllSpecEndpoints: as.ValidationAllSpecEndpoints,
	}
	if as.ValidationCustomList != nil {
		wire.ValidationCustomList = &xcValidationCustomList{
			EndpointValidationList: mapXCAPIOperations(as.ValidationCustomList.EndpointValidationList),
		}
	}
	return marshalJSON(wire)
}

func mapRingHash(rh *v1alpha1.RingHashConfig) json.RawMessage {
	wire := xcRingHashConfig{}
	for _, hp := range rh.HashPolicy {
		xhp := xcHashPolicy{HeaderName: hp.HeaderName, SourceIP: hp.SourceIP, Terminal: hp.Terminal}
		if hp.Cookie != nil {
			xhp.Cookie = &xcCookieForHashing{Name: hp.Cookie.Name, TTL: hp.Cookie.TTL, Path: hp.Cookie.Path}
		}
		wire.HashPolicy = append(wire.HashPolicy, xhp)
	}
	return marshalJSON(wire)
}
