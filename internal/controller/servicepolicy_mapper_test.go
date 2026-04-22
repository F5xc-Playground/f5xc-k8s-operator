package controller

import (
	"encoding/json"
	"testing"

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
			AllowAllRequests: &v1alpha1.EmptyObject{},
			AnyServer:        &v1alpha1.EmptyObject{},
		},
	}
	result := buildServicePolicyCreate(cr, "default")
	assert.Equal(t, "my-sp", result.Metadata.Name)
	assert.JSONEq(t, `{}`, string(result.Spec.AllowAllRequests))
	assert.JSONEq(t, `{}`, string(result.Spec.AnyServer))
}

func TestBuildServicePolicyCreate_AllowList(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-allow", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			AllowList: &v1alpha1.PolicyAllowDenyList{
				Prefixes:                []string{"10.0.0.0/8"},
				CountryList:             []string{"US", "GB"},
				DefaultActionNextPolicy: &v1alpha1.EmptyObject{},
			},
			AnyServer: &v1alpha1.EmptyObject{},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.NotNil(t, result.Spec.AllowList)
	assert.Nil(t, result.Spec.AllowAllRequests)

	var al map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.AllowList, &al))
	prefixes, ok := al["prefixes"].([]interface{})
	require.True(t, ok)
	assert.Len(t, prefixes, 1)
}

func TestBuildServicePolicyCreate_DenyList(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-deny", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			DenyList: &v1alpha1.PolicyAllowDenyList{
				Prefixes: []string{"192.168.0.0/16"},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.NotNil(t, result.Spec.DenyList)
	assert.Nil(t, result.Spec.AllowList)
}

func TestBuildServicePolicyCreate_RuleList(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-rules", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			RuleList: &v1alpha1.PolicyRuleList{
				Rules: []v1alpha1.PolicyRule{
					{
						Metadata: map[string]string{"name": "rule-1"},
						Spec: &v1alpha1.PolicyRuleSpec{
							Action:    "ALLOW",
							AnyClient: &v1alpha1.EmptyObject{},
						},
					},
				},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.NotNil(t, result.Spec.RuleList)
}

func TestBuildServicePolicyCreate_ServerNameChoice(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-sn", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &v1alpha1.EmptyObject{},
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
			AllowAllRequests: &v1alpha1.EmptyObject{},
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
			AllowAllRequests: &v1alpha1.EmptyObject{},
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
			AllowAllRequests: &v1alpha1.EmptyObject{},
		},
	}
	raw, err := buildServicePolicyDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasAllowAll := spec["allow_all_requests"]
	assert.True(t, hasAllowAll)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata)
}

func TestBuildServicePolicyCreate_ServerNameMatcher(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-snm", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &v1alpha1.EmptyObject{},
			ServerNameMatcher: &v1alpha1.ServerNameMatcher{
				ExactValues: []string{"api.example.com"},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, `{"exact_values":["api.example.com"]}`, string(result.Spec.ServerNameMatcher))
}

func TestBuildServicePolicyCreate_ServerSelector(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-ss", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &v1alpha1.EmptyObject{},
			ServerSelector: &v1alpha1.ServerSelector{
				Expressions: []string{"app in (web, api)"},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, `{"expressions":["app in (web, api)"]}`, string(result.Spec.ServerSelector))
}
