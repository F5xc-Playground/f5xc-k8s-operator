package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleAPIDefinition(name, namespace string) *v1alpha1.APIDefinition {
	return &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:  namespace,
			SwaggerSpecs: []string{"/api/object_store/namespaces/ns/stored_objects/swagger/petstore/v1"},
		},
	}
}

func TestBuildAPIDefinitionCreate_BasicFields(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "default")
	result := buildAPIDefinitionCreate(cr, "default")
	assert.Equal(t, "my-apidef", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildAPIDefinitionCreate_SwaggerSpecs(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	result := buildAPIDefinitionCreate(cr, "ns")

	var spec map[string]json.RawMessage
	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &spec))
	assert.Contains(t, string(spec["swagger_specs"]), "petstore")
}

func TestBuildAPIDefinitionCreate_MixedSchemaOrigin(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-mixed", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:       "ns",
			MixedSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"mixed_schema_origin":{}`)
	assert.NotContains(t, string(raw), "strict_schema_origin")
}

func TestBuildAPIDefinitionCreate_StrictSchemaOrigin(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-strict", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:        "ns",
			StrictSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"strict_schema_origin":{}`)
	assert.NotContains(t, string(raw), "mixed_schema_origin")
}

func TestBuildAPIDefinitionCreate_InventoryLists(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-inv", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace: "ns",
			APIInventoryInclusionList: []v1alpha1.APIOperation{
				{Method: "GET", Path: "/api/users"},
			},
			APIInventoryExclusionList: []v1alpha1.APIOperation{
				{Method: "DELETE", Path: "/api/admin"},
			},
			NonAPIEndpoints: []v1alpha1.APIOperation{
				{Method: "GET", Path: "/health"},
			},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"api_inventory_inclusion_list"`)
	assert.Contains(t, string(raw), `"api_inventory_exclusion_list"`)
	assert.Contains(t, string(raw), `"non_api_endpoints"`)
}

func TestBuildAPIDefinitionReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	result := buildAPIDefinitionReplace(cr, "ns", "rv-3")
	assert.Equal(t, "rv-3", result.Metadata.ResourceVersion)
}

func TestBuildAPIDefinitionDesiredSpecJSON(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	raw, err := buildAPIDefinitionDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasSwagger := spec["swagger_specs"]
	assert.True(t, hasSwagger)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
