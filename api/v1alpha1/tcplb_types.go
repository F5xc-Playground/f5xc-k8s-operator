package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tlb
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// TCPLoadBalancer is the Schema for the tcploadbalancers API.
type TCPLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TCPLoadBalancerSpec   `json:"spec,omitempty"`
	Status TCPLoadBalancerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TCPLoadBalancerList contains a list of TCPLoadBalancer.
type TCPLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TCPLoadBalancer `json:"items"`
}

// TCPLoadBalancerSpec defines the desired state of a TCPLoadBalancer.
type TCPLoadBalancerSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string      `json:"xcNamespace"`
	Domains     []string    `json:"domains"`
	ListenPort  uint32      `json:"listenPort"`
	OriginPools []RoutePool `json:"originPools"`

	// TLS OneOf: noTLS, tlsParameters, tlsTCPAutoCert
	NoTLS          *EmptyObject    `json:"noTLS,omitempty"`
	TLSParameters  *TLSParameters  `json:"tlsParameters,omitempty"`
	TLSTCPAutoCert *TLSTCPAutoCert `json:"tlsTCPAutoCert,omitempty"`

	// Advertise OneOf: advertiseOnPublicDefaultVIP, advertiseOnPublic, advertiseCustom, doNotAdvertise
	AdvertiseOnPublicDefaultVIP *EmptyObject       `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *AdvertiseOnPublic `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *AdvertiseCustom   `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *EmptyObject       `json:"doNotAdvertise,omitempty"`
}

// TLSParameters holds typed TLS configuration for a TCP load balancer.
type TLSParameters struct {
	TLSCertificates []TLSCertificateRef `json:"tlsCertificates,omitempty"`
	DefaultSecurity *EmptyObject        `json:"defaultSecurity,omitempty"`
	LowSecurity     *EmptyObject        `json:"lowSecurity,omitempty"`
	MediumSecurity  *EmptyObject        `json:"mediumSecurity,omitempty"`
	CustomSecurity  *CustomTLSSecurity  `json:"customSecurity,omitempty"`
	NoMTLS          *EmptyObject        `json:"noMTLS,omitempty"`
	UseMTLS         *UseMTLS            `json:"useMTLS,omitempty"`
}

// TLSTCPAutoCert holds mTLS configuration for auto-cert TCP load balancers.
type TLSTCPAutoCert struct {
	NoMTLS  *EmptyObject `json:"noMTLS,omitempty"`
	UseMTLS *UseMTLS     `json:"useMTLS,omitempty"`
}

// TCPLoadBalancerStatus defines the observed state of TCPLoadBalancer.
type TCPLoadBalancerStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&TCPLoadBalancer{}, &TCPLoadBalancerList{})
}
