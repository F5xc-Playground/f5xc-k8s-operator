package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	// Rule choice OneOf: allowAllRequests, allowList, denyAllRequests, denyList, ruleList
	AllowAllRequests *apiextensionsv1.JSON `json:"allowAllRequests,omitempty"`
	AllowList        *apiextensionsv1.JSON `json:"allowList,omitempty"`
	DenyAllRequests  *apiextensionsv1.JSON `json:"denyAllRequests,omitempty"`
	DenyList         *apiextensionsv1.JSON `json:"denyList,omitempty"`
	RuleList         *apiextensionsv1.JSON `json:"ruleList,omitempty"`

	// Server choice OneOf: anyServer, serverName, serverNameMatcher, serverSelector
	AnyServer         *apiextensionsv1.JSON `json:"anyServer,omitempty"`
	ServerName        string                `json:"serverName,omitempty"`
	ServerNameMatcher *apiextensionsv1.JSON `json:"serverNameMatcher,omitempty"`
	ServerSelector    *apiextensionsv1.JSON `json:"serverSelector,omitempty"`
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
