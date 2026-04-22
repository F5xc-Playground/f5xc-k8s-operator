package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=uid
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// UserIdentification is the Schema for the useridentifications API.
type UserIdentification struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserIdentificationSpec   `json:"spec,omitempty"`
	Status UserIdentificationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserIdentificationList contains a list of UserIdentification.
type UserIdentificationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserIdentification `json:"items"`
}

type UserIdentificationSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string                   `json:"xcNamespace"`
	Rules       []UserIdentificationRule `json:"rules"`
}

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

type UserIdentificationStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&UserIdentification{}, &UserIdentificationList{})
}
