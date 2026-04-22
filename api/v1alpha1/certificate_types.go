package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cert
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Certificate is the Schema for the certificates API.
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec   `json:"spec,omitempty"`
	Status CertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CertificateList contains a list of Certificate.
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

type CertificateSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// SecretRef points to a kubernetes.io/tls Secret containing tls.crt and tls.key.
	// +kubebuilder:validation:Required
	SecretRef SecretRef `json:"secretRef"`

	// OCSP stapling choice — all optional; omitting defaults to disabled at the API level.
	// OneOf: CustomHashAlgorithms | DisableOcspStapling | UseSystemDefaults
	CustomHashAlgorithms *CustomHashAlgorithms `json:"customHashAlgorithms,omitempty"`
	DisableOcspStapling  *EmptyObject          `json:"disableOcspStapling,omitempty"`
	UseSystemDefaults    *EmptyObject          `json:"useSystemDefaults,omitempty"`
}

// CustomHashAlgorithms specifies a list of hash algorithms for certificate OCSP stapling.
type CustomHashAlgorithms struct {
	HashAlgorithms []string `json:"hashAlgorithms"`
}

type SecretRef struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the Secret. Defaults to the Certificate CR's namespace.
	Namespace string `json:"namespace,omitempty"`
}

type CertificateStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}
