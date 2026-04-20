package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	FinalizerXCCleanup = "xc.f5.com/cleanup"

	AnnotationXCNamespace    = "f5xc.io/namespace"
	AnnotationDeletionPolicy = "f5xc.io/deletion-policy"

	DeletionPolicyOrphan = "orphan"

	ConditionReady  = "Ready"
	ConditionSynced = "Synced"

	ReasonCreateSucceeded = "CreateSucceeded"
	ReasonUpdateSucceeded = "UpdateSucceeded"
	ReasonUpToDate        = "UpToDate"
	ReasonDeleteSucceeded = "DeleteSucceeded"
	ReasonCreateFailed    = "CreateFailed"
	ReasonUpdateFailed    = "UpdateFailed"
	ReasonDeleteFailed    = "DeleteFailed"
	ReasonAuthFailure     = "AuthFailure"
	ReasonRateLimited     = "RateLimited"
	ReasonServerError     = "ServerError"
	ReasonConflict        = "Conflict"
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
	OriginServers         []OriginServer        `json:"originServers"`
	Port                  int                   `json:"port"`
	LoadBalancerAlgorithm string                `json:"loadBalancerAlgorithm,omitempty"`
	HealthChecks          []ObjectRef           `json:"healthChecks,omitempty"`
	UseTLS                *apiextensionsv1.JSON `json:"useTLS,omitempty"`
	NoTLS                 *apiextensionsv1.JSON `json:"noTLS,omitempty"`
}

type OriginPoolStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

type OriginServer struct {
	PublicIP      *PublicIP      `json:"publicIP,omitempty"`
	PublicName    *PublicName    `json:"publicName,omitempty"`
	PrivateIP     *PrivateIP     `json:"privateIP,omitempty"`
	PrivateName   *PrivateName   `json:"privateName,omitempty"`
	K8SService    *K8SService    `json:"k8sService,omitempty"`
	ConsulService *ConsulService `json:"consulService,omitempty"`
}

type PublicIP struct {
	IP string `json:"ip"`
}

type PublicName struct {
	DNSName string `json:"dnsName"`
}

type PrivateIP struct {
	IP   string     `json:"ip"`
	Site *ObjectRef `json:"site,omitempty"`
}

type PrivateName struct {
	DNSName string     `json:"dnsName"`
	Site    *ObjectRef `json:"site,omitempty"`
}

type K8SService struct {
	ServiceName      string     `json:"serviceName"`
	ServiceNamespace string     `json:"serviceNamespace,omitempty"`
	Site             *ObjectRef `json:"site,omitempty"`
}

type ConsulService struct {
	ServiceName string     `json:"serviceName"`
	Site        *ObjectRef `json:"site,omitempty"`
}

type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

func init() {
	SchemeBuilder.Register(&OriginPool{}, &OriginPoolList{})
}
