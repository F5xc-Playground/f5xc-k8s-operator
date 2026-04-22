package v1alpha1

import (
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
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// Detection — OneOf: DefaultDetectionSettings | DetectionSettings
	DefaultDetectionSettings *EmptyObject       `json:"defaultDetectionSettings,omitempty"`
	DetectionSettings        *DetectionSettings `json:"detectionSettings,omitempty"`

	// Enforcement mode — OneOf: Monitoring | Blocking
	Monitoring *EmptyObject `json:"monitoring,omitempty"`
	Blocking   *EmptyObject `json:"blocking,omitempty"`

	// Blocking page — OneOf: UseDefaultBlockingPage | BlockingPage
	UseDefaultBlockingPage *EmptyObject  `json:"useDefaultBlockingPage,omitempty"`
	BlockingPage           *BlockingPage `json:"blockingPage,omitempty"`

	// Response codes — OneOf: AllowAllResponseCodes | AllowedResponseCodes
	AllowAllResponseCodes *EmptyObject          `json:"allowAllResponseCodes,omitempty"`
	AllowedResponseCodes  *AllowedResponseCodes `json:"allowedResponseCodes,omitempty"`

	// Bot setting — OneOf: DefaultBotSetting | BotProtectionSetting
	DefaultBotSetting    *EmptyObject          `json:"defaultBotSetting,omitempty"`
	BotProtectionSetting *BotProtectionSetting `json:"botProtectionSetting,omitempty"`

	// Anonymization — OneOf: DefaultAnonymization | DisableAnonymization | CustomAnonymization
	DefaultAnonymization *EmptyObject         `json:"defaultAnonymization,omitempty"`
	DisableAnonymization *EmptyObject         `json:"disableAnonymization,omitempty"`
	CustomAnonymization  *CustomAnonymization `json:"customAnonymization,omitempty"`
}

type DetectionSettings struct {
	SignatureSelectionSetting *SignatureSelectionSetting `json:"signatureSelectionSetting,omitempty"`
	EnableSuppression        *EmptyObject               `json:"enableSuppression,omitempty"`
	DisableSuppression       *EmptyObject               `json:"disableSuppression,omitempty"`
	EnableThreatCampaigns    *EmptyObject               `json:"enableThreatCampaigns,omitempty"`
	DisableThreatCampaigns   *EmptyObject               `json:"disableThreatCampaigns,omitempty"`
}

type SignatureSelectionSetting struct {
	DefaultAttackTypeSettings       *EmptyObject        `json:"defaultAttackTypeSettings,omitempty"`
	AttackTypeSettings              *AttackTypeSettings `json:"attackTypeSettings,omitempty"`
	HighMediumLowAccuracySignatures *EmptyObject        `json:"highMediumLowAccuracySignatures,omitempty"`
	HighMediumAccuracySignatures    *EmptyObject        `json:"highMediumAccuracySignatures,omitempty"`
	OnlyHighAccuracySignatures      *EmptyObject        `json:"onlyHighAccuracySignatures,omitempty"`
}

type AttackTypeSettings struct {
	DisabledAttackTypes []AttackType `json:"disabledAttackTypes,omitempty"`
}

type AttackType struct {
	Name string `json:"name"`
}

type BlockingPage struct {
	BlockingPage string `json:"blockingPage,omitempty"`
	ResponseCode string `json:"responseCode,omitempty"`
}

type AllowedResponseCodes struct {
	ResponseCode []int `json:"responseCode"`
}

type BotProtectionSetting struct {
	MaliciousBotAction  string `json:"maliciousBotAction,omitempty"`
	SuspiciousBotAction string `json:"suspiciousBotAction,omitempty"`
	GoodBotAction       string `json:"goodBotAction,omitempty"`
}

type CustomAnonymization struct {
	AnonymizationConfig []AnonymizationEntry `json:"anonymizationConfig,omitempty"`
	SpecificDomains     []string             `json:"specificDomains,omitempty"`
}

type AnonymizationEntry struct {
	HeaderName     string `json:"headerName,omitempty"`
	QueryParameter string `json:"queryParameter,omitempty"`
	CookieName     string `json:"cookieName,omitempty"`
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
