package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hc
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

type HealthCheckSpec struct {
	HTTPHealthCheck    *HTTPHealthCheckSpec `json:"httpHealthCheck,omitempty"`
	TCPHealthCheck     *TCPHealthCheckSpec  `json:"tcpHealthCheck,omitempty"`
	HealthyThreshold   *uint32              `json:"healthyThreshold,omitempty"`
	UnhealthyThreshold *uint32              `json:"unhealthyThreshold,omitempty"`
	Interval           *uint32              `json:"interval,omitempty"`
	Timeout            *uint32              `json:"timeout,omitempty"`
	JitterPercent      *uint32              `json:"jitterPercent,omitempty"`
}

type HTTPHealthCheckSpec struct {
	Path                string   `json:"path,omitempty"`
	UseHTTP2            bool     `json:"useHTTP2,omitempty"`
	ExpectedStatusCodes []string `json:"expectedStatusCodes,omitempty"`
}

type TCPHealthCheckSpec struct {
	Send    string `json:"send,omitempty"`
	Receive string `json:"receive,omitempty"`
}

type HealthCheckStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
