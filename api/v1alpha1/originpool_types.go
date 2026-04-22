package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=op
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OriginPool is the Schema for the originpools API.
type OriginPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OriginPoolSpec   `json:"spec,omitempty"`
	Status OriginPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OriginPoolList contains a list of OriginPool.
type OriginPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OriginPool `json:"items"`
}

type OriginPoolSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace           string                `json:"xcNamespace"`
	OriginServers         []OriginServer        `json:"originServers"`
	Port                  int                   `json:"port"`
	LoadBalancerAlgorithm string                `json:"loadBalancerAlgorithm,omitempty"`
	HealthChecks          []ObjectRef           `json:"healthChecks,omitempty"`
	UseTLS                *OriginPoolTLS `json:"useTLS,omitempty"`
	NoTLS                 *EmptyObject   `json:"noTLS,omitempty"`
}

type OriginPoolStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
	DiscoveredOrigins  []DiscoveredOrigin `json:"discoveredOrigins,omitempty"`
}

type DiscoveredOrigin struct {
	Resource    ResourceRef `json:"resource"`
	Address     string      `json:"address,omitempty"`
	Port        uint32      `json:"port,omitempty"`
	AddressType string      `json:"addressType,omitempty"`
	Status      string      `json:"status"`
	Message     string      `json:"message,omitempty"`
}

type OriginPoolTLS struct {
	DefaultSecurity        *EmptyObject       `json:"defaultSecurity,omitempty"`
	LowSecurity            *EmptyObject       `json:"lowSecurity,omitempty"`
	MediumSecurity         *EmptyObject       `json:"mediumSecurity,omitempty"`
	CustomSecurity         *CustomTLSSecurity `json:"customSecurity,omitempty"`
	SNI                    string             `json:"sni,omitempty"`
	VolterraTrustedCA      *EmptyObject       `json:"volterraTrustedCA,omitempty"`
	TrustedCAURL           string             `json:"trustedCAURL,omitempty"`
	DisableSNI             *EmptyObject       `json:"disableSNI,omitempty"`
	UseServerVerification  *EmptyObject       `json:"useServerVerification,omitempty"`
	SkipServerVerification *EmptyObject       `json:"skipServerVerification,omitempty"`
	NoMTLS                 *EmptyObject       `json:"noMTLS,omitempty"`
}

type OriginServer struct {
	PublicIP      *PublicIP             `json:"publicIP,omitempty"`
	PublicName    *PublicName           `json:"publicName,omitempty"`
	PrivateIP     *PrivateIP            `json:"privateIP,omitempty"`
	PrivateName   *PrivateName          `json:"privateName,omitempty"`
	K8SService    *K8SService           `json:"k8sService,omitempty"`
	ConsulService *ConsulService        `json:"consulService,omitempty"`
	Discover      *OriginServerDiscover `json:"discover,omitempty"`
}

type PublicIP struct {
	IP string `json:"ip"`
}

type PublicName struct {
	DNSName string `json:"dnsName"`
}

type PrivateIP struct {
	IP             string       `json:"ip"`
	Site           *ObjectRef   `json:"site,omitempty"`
	VirtualSite    *ObjectRef   `json:"virtualSite,omitempty"`
	InsideNetwork  *EmptyObject `json:"insideNetwork,omitempty"`
	OutsideNetwork *EmptyObject `json:"outsideNetwork,omitempty"`
}

type PrivateName struct {
	DNSName        string       `json:"dnsName"`
	Site           *ObjectRef   `json:"site,omitempty"`
	VirtualSite    *ObjectRef   `json:"virtualSite,omitempty"`
	InsideNetwork  *EmptyObject `json:"insideNetwork,omitempty"`
	OutsideNetwork *EmptyObject `json:"outsideNetwork,omitempty"`
}

type K8SService struct {
	ServiceName      string       `json:"serviceName"`
	ServiceNamespace string       `json:"serviceNamespace,omitempty"`
	Site             *ObjectRef   `json:"site,omitempty"`
	VirtualSite      *ObjectRef   `json:"virtualSite,omitempty"`
	InsideNetwork    *EmptyObject `json:"insideNetwork,omitempty"`
	OutsideNetwork   *EmptyObject `json:"outsideNetwork,omitempty"`
}

type ConsulService struct {
	ServiceName    string       `json:"serviceName"`
	Site           *ObjectRef   `json:"site,omitempty"`
	VirtualSite    *ObjectRef   `json:"virtualSite,omitempty"`
	InsideNetwork  *EmptyObject `json:"insideNetwork,omitempty"`
	OutsideNetwork *EmptyObject `json:"outsideNetwork,omitempty"`
}

type OriginServerDiscover struct {
	Resource        ResourceRef `json:"resource"`
	AddressOverride string      `json:"addressOverride,omitempty"`
	PortOverride    *uint32     `json:"portOverride,omitempty"`
}

func init() {
	SchemeBuilder.Register(&OriginPool{}, &OriginPoolList{})
}
