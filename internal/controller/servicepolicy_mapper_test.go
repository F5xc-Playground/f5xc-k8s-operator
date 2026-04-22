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
			XCNamespace:      "default",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
			AnyServer:        &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}

	result := buildServicePolicyCreate(cr, "default")
	assert.Equal(t, "my-sp", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.JSONEq(t, `{}`, string(result.Spec.AllowAllRequests))
	assert.JSONEq(t, `{}`, string(result.Spec.AnyServer))
}

func TestBuildServicePolicyCreate_AllowList(t *testing.T) {
	allowListJSON := `{"rules":[{"metadata":{"name":"rule-1"}}]}`
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-allow", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			AllowList:   &apiextensionsv1.JSON{Raw: json.RawMessage(allowListJSON)},
			AnyServer:   &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}

	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, allowListJSON, string(result.Spec.AllowList))
	assert.Nil(t, result.Spec.AllowAllRequests)
	assert.Nil(t, result.Spec.DenyAllRequests)
	assert.Nil(t, result.Spec.DenyList)
	assert.Nil(t, result.Spec.RuleList)
}

func TestBuildServicePolicyCreate_DenyList(t *testing.T) {
	denyListJSON := `{"rules":[{"metadata":{"name":"deny-rule"}}]}`
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-deny", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			DenyList:    &apiextensionsv1.JSON{Raw: json.RawMessage(denyListJSON)},
		},
	}

	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, denyListJSON, string(result.Spec.DenyList))
	assert.Nil(t, result.Spec.AllowList)
}

func TestBuildServicePolicyCreate_RuleList(t *testing.T) {
	ruleListJSON := `{"rules":[{"metadata":{"name":"custom-rule"}}]}`
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-rules", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			RuleList:    &apiextensionsv1.JSON{Raw: json.RawMessage(ruleListJSON)},
		},
	}

	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, ruleListJSON, string(result.Spec.RuleList))
}

func TestBuildServicePolicyCreate_ServerNameChoice(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-sn", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
			ServerName:       "app.example.com",
		},
	}

	result := buildServicePolicyCreate(cr, "ns")
	assert.Equal(t, "app.example.com", result.Spec.ServerName)
	assert.Nil(t, result.Spec.AnyServer)
	assert.Nil(t, result.Spec.ServerSelector)
}

func TestBuildServicePolicyReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}

	result := buildServicePolicyReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
}

func TestBuildServicePolicyCreate_XCNamespaceOverride(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "k8s-ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "k8s-ns",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}

	result := buildServicePolicyCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildServicePolicyDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}

	raw, err := buildServicePolicyDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasAllowAll := spec["allow_all_requests"]
	assert.True(t, hasAllowAll)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
