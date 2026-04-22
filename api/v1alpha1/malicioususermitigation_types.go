package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mum
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MaliciousUserMitigation is the Schema for the malicioususermitigations API.
type MaliciousUserMitigation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaliciousUserMitigationSpec   `json:"spec,omitempty"`
	Status MaliciousUserMitigationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MaliciousUserMitigationList contains a list of MaliciousUserMitigation.
type MaliciousUserMitigationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaliciousUserMitigation `json:"items"`
}

type MaliciousUserMitigationSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace    string                       `json:"xcNamespace"`
	MitigationType *MaliciousUserMitigationType `json:"mitigationType,omitempty"`
}

type MaliciousUserMitigationType struct {
	Rules []MaliciousUserMitigationRule `json:"rules"`
}

type MaliciousUserMitigationRule struct {
	ThreatLevel      MaliciousUserThreatLevel      `json:"threatLevel"`
	MitigationAction MaliciousUserMitigationAction `json:"mitigationAction"`
}

type MaliciousUserThreatLevel struct {
	Low    *EmptyObject `json:"low,omitempty"`
	Medium *EmptyObject `json:"medium,omitempty"`
	High   *EmptyObject `json:"high,omitempty"`
}

type MaliciousUserMitigationAction struct {
	BlockTemporarily    *EmptyObject `json:"blockTemporarily,omitempty"`
	CaptchaChallenge    *EmptyObject `json:"captchaChallenge,omitempty"`
	JavascriptChallenge *EmptyObject `json:"javascriptChallenge,omitempty"`
}

type MaliciousUserMitigationStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MaliciousUserMitigation{}, &MaliciousUserMitigationList{})
}
