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

func sampleAppFirewall(name, namespace string) *v1alpha1.AppFirewall {
	return &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}
}

func TestBuildAppFirewallCreate_BasicFields(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "default")

	result := buildAppFirewallCreate(cr, "default")
	assert.Equal(t, "my-afw", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.JSONEq(t, "{}", string(result.Spec.Blocking))
	assert.Nil(t, result.Spec.Monitoring)
}

func TestBuildAppFirewallCreate_MultipleOneOfGroups(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-multi", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking:             &apiextensionsv1.JSON{Raw: []byte(`{"mode":"blocking"}`)},
			UseDefaultBlockingPage: &apiextensionsv1.JSON{Raw: []byte(`{}`)},
			DefaultBotSetting:    &apiextensionsv1.JSON{Raw: []byte(`{}`)},
			DefaultAnonymization: &apiextensionsv1.JSON{Raw: []byte(`{}`)},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{"mode":"blocking"}`, string(result.Spec.Blocking))
	assert.JSONEq(t, `{}`, string(result.Spec.UseDefaultBlockingPage))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultBotSetting))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultAnonymization))
	assert.Nil(t, result.Spec.Monitoring)
	assert.Nil(t, result.Spec.BlockingPage)
}

func TestBuildAppFirewallReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "ns")

	result := buildAppFirewallReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.JSONEq(t, "{}", string(result.Spec.Blocking))
}

func TestBuildAppFirewallCreate_XCNamespaceOverride(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "k8s-ns")

	result := buildAppFirewallCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildAppFirewallDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "my-afw", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &apiextensionsv1.JSON{Raw: []byte(`{"mode":"blocking"}`)},
		},
	}

	raw, err := buildAppFirewallDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasBlocking := spec["blocking"]
	assert.True(t, hasBlocking)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
