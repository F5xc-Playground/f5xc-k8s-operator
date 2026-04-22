package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=apidef
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// APIDefinition is the Schema for the apidefinitions API.
type APIDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIDefinitionSpec   `json:"spec,omitempty"`
	Status APIDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// APIDefinitionList contains a list of APIDefinition.
type APIDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIDefinition `json:"items"`
}

type APIDefinitionSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace               string         `json:"xcNamespace"`
	SwaggerSpecs              []string       `json:"swaggerSpecs,omitempty"`
	APIInventoryInclusionList []APIOperation `json:"apiInventoryInclusionList,omitempty"`
	APIInventoryExclusionList []APIOperation `json:"apiInventoryExclusionList,omitempty"`
	NonAPIEndpoints           []APIOperation `json:"nonAPIEndpoints,omitempty"`

	// OneOf: schema_updates_strategy
	MixedSchemaOrigin  *EmptyObject `json:"mixedSchemaOrigin,omitempty"`
	StrictSchemaOrigin *EmptyObject `json:"strictSchemaOrigin,omitempty"`
}

type APIDefinitionStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&APIDefinition{}, &APIDefinitionList{})
}
