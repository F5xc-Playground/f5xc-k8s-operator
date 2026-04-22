package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sp
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ServicePolicy is the Schema for the servicepolicies API.
type ServicePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServicePolicySpec   `json:"spec,omitempty"`
	Status ServicePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServicePolicyList contains a list of ServicePolicy.
type ServicePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServicePolicy `json:"items"`
}

type ServicePolicySpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// Rule choice OneOf
	AllowAllRequests *EmptyObject        `json:"allowAllRequests,omitempty"`
	AllowList        *PolicyAllowDenyList `json:"allowList,omitempty"`
	DenyAllRequests  *EmptyObject        `json:"denyAllRequests,omitempty"`
	DenyList         *PolicyAllowDenyList `json:"denyList,omitempty"`
	RuleList         *PolicyRuleList     `json:"ruleList,omitempty"`

	// Server choice OneOf
	AnyServer         *EmptyObject       `json:"anyServer,omitempty"`
	ServerName        string             `json:"serverName,omitempty"`
	ServerNameMatcher *ServerNameMatcher `json:"serverNameMatcher,omitempty"`
	ServerSelector    *ServerSelector    `json:"serverSelector,omitempty"`
}

type PolicyAllowDenyList struct {
	Prefixes                []string     `json:"prefixes,omitempty"`
	IPPrefixSet             []ObjectRef  `json:"ipPrefixSet,omitempty"`
	ASNList                 *ASNList     `json:"asnList,omitempty"`
	ASNSet                  []ObjectRef  `json:"asnSet,omitempty"`
	CountryList             []string     `json:"countryList,omitempty"`
	DefaultActionNextPolicy *EmptyObject `json:"defaultActionNextPolicy,omitempty"`
	DefaultActionDeny       *EmptyObject `json:"defaultActionDeny,omitempty"`
	DefaultActionAllow      *EmptyObject `json:"defaultActionAllow,omitempty"`
}

type ASNList struct {
	ASNumbers []uint32 `json:"asNumbers"`
}

type PolicyRuleList struct {
	Rules []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Spec     *PolicyRuleSpec   `json:"spec,omitempty"`
}

type PolicyRuleSpec struct {
	Action         string             `json:"action,omitempty"`
	AnyClient      *EmptyObject       `json:"anyClient,omitempty"`
	ClientName     string             `json:"clientName,omitempty"`
	ClientSelector *LabelSelector     `json:"clientSelector,omitempty"`
	IPMatcher      *IPMatcher         `json:"ipMatcher,omitempty"`
	ASNMatcher     *ASNMatcher        `json:"asnMatcher,omitempty"`
	Path           *PathMatcher       `json:"path,omitempty"`
	Headers        []HeaderMatcher    `json:"headers,omitempty"`
	HTTPMethod     *HTTPMethodMatcher `json:"httpMethod,omitempty"`
}

type ServerNameMatcher struct {
	ExactValues []string `json:"exactValues,omitempty"`
	RegexValues []string `json:"regexValues,omitempty"`
}

type ServerSelector struct {
	Expressions []string `json:"expressions,omitempty"`
}

type ServicePolicyStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ServicePolicy{}, &ServicePolicyList{})
}
