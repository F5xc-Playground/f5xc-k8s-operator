package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=afw
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AppFirewall is the Schema for the appfirewalls API.
type AppFirewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppFirewallSpec   `json:"spec,omitempty"`
	Status AppFirewallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppFirewallList contains a list of AppFirewall.
type AppFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppFirewall `json:"items"`
}

type AppFirewallSpec struct {
	// Detection — OneOf: DefaultDetectionSettings | DetectionSettings
	DefaultDetectionSettings *apiextensionsv1.JSON `json:"defaultDetectionSettings,omitempty"`
	DetectionSettings        *apiextensionsv1.JSON `json:"detectionSettings,omitempty"`

	// Enforcement mode — OneOf: Monitoring | Blocking
	Monitoring *apiextensionsv1.JSON `json:"monitoring,omitempty"`
	Blocking   *apiextensionsv1.JSON `json:"blocking,omitempty"`

	// Blocking page — OneOf: UseDefaultBlockingPage | BlockingPage
	UseDefaultBlockingPage *apiextensionsv1.JSON `json:"useDefaultBlockingPage,omitempty"`
	BlockingPage           *apiextensionsv1.JSON `json:"blockingPage,omitempty"`

	// Response codes — OneOf: AllowAllResponseCodes | AllowedResponseCodes
	AllowAllResponseCodes *apiextensionsv1.JSON `json:"allowAllResponseCodes,omitempty"`
	AllowedResponseCodes  *apiextensionsv1.JSON `json:"allowedResponseCodes,omitempty"`

	// Bot setting — OneOf: DefaultBotSetting | BotProtectionSetting
	DefaultBotSetting    *apiextensionsv1.JSON `json:"defaultBotSetting,omitempty"`
	BotProtectionSetting *apiextensionsv1.JSON `json:"botProtectionSetting,omitempty"`

	// Anonymization — OneOf: DefaultAnonymization | CustomAnonymization
	DefaultAnonymization *apiextensionsv1.JSON `json:"defaultAnonymization,omitempty"`
	CustomAnonymization  *apiextensionsv1.JSON `json:"customAnonymization,omitempty"`

	// Loadbalancer setting
	UseLoadbalancerSetting *apiextensionsv1.JSON `json:"useLoadbalancerSetting,omitempty"`
}

type AppFirewallStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AppFirewall{}, &AppFirewallList{})
}
