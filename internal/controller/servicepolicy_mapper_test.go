package controller

import (
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildServicePolicyCreate_BasicFields(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "default"},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
		},
	}

	result := buildServicePolicyCreate(cr, "default")
	assert.Equal(t, "my-sp", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, "FIRST_MATCH", result.Spec.Algo)
	assert.Empty(t, result.Spec.Rules)
}

func TestBuildServicePolicyCreate_WithRules(t *testing.T) {
	rule1 := `{"action":"ALLOW","name":"rule-1"}`
	rule2 := `{"action":"DENY","name":"rule-2"}`
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-rules", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
			Rules: []apiextensionsv1.JSON{
				{Raw: json.RawMessage(rule1)},
				{Raw: json.RawMessage(rule2)},
			},
		},
	}

	result := buildServicePolicyCreate(cr, "ns")
	require.Len(t, result.Spec.Rules, 2)
	assert.JSONEq(t, rule1, string(result.Spec.Rules[0]))
	assert.JSONEq(t, rule2, string(result.Spec.Rules[1]))
}

func TestBuildServicePolicyReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
		},
	}

	result := buildServicePolicyReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.Equal(t, "FIRST_MATCH", result.Spec.Algo)
}

func TestBuildServicePolicyCreate_XCNamespaceOverride(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "k8s-ns"},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
		},
	}

	result := buildServicePolicyCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildServicePolicyDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
		},
	}

	raw, err := buildServicePolicyDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasAlgo := spec["algo"]
	assert.True(t, hasAlgo)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
