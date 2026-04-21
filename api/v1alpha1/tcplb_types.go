package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	Domains     []string    `json:"domains"`
	ListenPort  uint32      `json:"listenPort"`
	OriginPools []RoutePool `json:"originPools"`

	// TLS OneOf: noTLS, tlsParameters, tlsPassthrough
	NoTLS          *apiextensionsv1.JSON `json:"noTLS,omitempty"`
	TLSParameters  *apiextensionsv1.JSON `json:"tlsParameters,omitempty"`
	TLSPassthrough *apiextensionsv1.JSON `json:"tlsPassthrough,omitempty"`

	// Advertise OneOf: advertiseOnPublicDefaultVIP, advertiseOnPublic, advertiseCustom, doNotAdvertise
	AdvertiseOnPublicDefaultVIP *apiextensionsv1.JSON `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *apiextensionsv1.JSON `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *apiextensionsv1.JSON `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *apiextensionsv1.JSON `json:"doNotAdvertise,omitempty"`
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
