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
	Domains           []string              `json:"domains"`
	DefaultRoutePools []RoutePool           `json:"defaultRoutePools"`
	Routes            []apiextensionsv1.JSON `json:"routes,omitempty"`

	// TLS OneOf: http, https, httpsAutoCert
	HTTP          *apiextensionsv1.JSON `json:"http,omitempty"`
	HTTPS         *apiextensionsv1.JSON `json:"https,omitempty"`
	HTTPSAutoCert *apiextensionsv1.JSON `json:"httpsAutoCert,omitempty"`

	// WAF OneOf: disableWAF, appFirewall
	DisableWAF  *apiextensionsv1.JSON `json:"disableWAF,omitempty"`
	AppFirewall *ObjectRef            `json:"appFirewall,omitempty"`

	// Bot defense OneOf: disableBotDefense, botDefense
	DisableBotDefense *apiextensionsv1.JSON `json:"disableBotDefense,omitempty"`
	BotDefense        *apiextensionsv1.JSON `json:"botDefense,omitempty"`

	// API discovery OneOf: disableAPIDiscovery, enableAPIDiscovery
	DisableAPIDiscovery *apiextensionsv1.JSON `json:"disableAPIDiscovery,omitempty"`
	EnableAPIDiscovery  *apiextensionsv1.JSON `json:"enableAPIDiscovery,omitempty"`

	// IP reputation OneOf: disableIPReputation, enableIPReputation
	DisableIPReputation *apiextensionsv1.JSON `json:"disableIPReputation,omitempty"`
	EnableIPReputation  *apiextensionsv1.JSON `json:"enableIPReputation,omitempty"`

	// Rate limit OneOf: disableRateLimit, rateLimit
	DisableRateLimit *apiextensionsv1.JSON `json:"disableRateLimit,omitempty"`
	RateLimit        *apiextensionsv1.JSON `json:"rateLimit,omitempty"`

	// Challenge OneOf: noChallenge, jsChallenge, captchaChallenge, policyBasedChallenge
	NoChallenge          *apiextensionsv1.JSON `json:"noChallenge,omitempty"`
	JSChallenge          *apiextensionsv1.JSON `json:"jsChallenge,omitempty"`
	CaptchaChallenge     *apiextensionsv1.JSON `json:"captchaChallenge,omitempty"`
	PolicyBasedChallenge *apiextensionsv1.JSON `json:"policyBasedChallenge,omitempty"`

	// LB algorithm OneOf: roundRobin, leastActive, random, sourceIPStickiness,
	// cookieStickiness, ringHash
	RoundRobin         *apiextensionsv1.JSON `json:"roundRobin,omitempty"`
	LeastActive        *apiextensionsv1.JSON `json:"leastActive,omitempty"`
	Random             *apiextensionsv1.JSON `json:"random,omitempty"`
	SourceIPStickiness *apiextensionsv1.JSON `json:"sourceIPStickiness,omitempty"`
	CookieStickiness   *apiextensionsv1.JSON `json:"cookieStickiness,omitempty"`
	RingHash           *apiextensionsv1.JSON `json:"ringHash,omitempty"`

	// Advertise OneOf: advertiseOnPublicDefaultVIP, advertiseOnPublic,
	// advertiseCustom, doNotAdvertise
	AdvertiseOnPublicDefaultVIP *apiextensionsv1.JSON `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *apiextensionsv1.JSON `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *apiextensionsv1.JSON `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *apiextensionsv1.JSON `json:"doNotAdvertise,omitempty"`

	// Service policies OneOf: servicePoliciesFromNamespace, activeServicePolicies,
	// noServicePolicies
	ServicePoliciesFromNamespace *apiextensionsv1.JSON `json:"servicePoliciesFromNamespace,omitempty"`
	ActiveServicePolicies        *apiextensionsv1.JSON `json:"activeServicePolicies,omitempty"`
	NoServicePolicies            *apiextensionsv1.JSON `json:"noServicePolicies,omitempty"`

	// User ID OneOf: userIDClientIP
	UserIDClientIP *apiextensionsv1.JSON `json:"userIDClientIP,omitempty"`
}

// HTTPLoadBalancerStatus defines the observed state of HTTPLoadBalancer.
type HTTPLoadBalancerStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HTTPLoadBalancer{}, &HTTPLoadBalancerList{})
}
