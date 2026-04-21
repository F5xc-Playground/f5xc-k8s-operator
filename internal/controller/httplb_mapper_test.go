package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sampleHTTPLoadBalancer(name, namespace string) *v1alpha1.HTTPLoadBalancer {
	return &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			Domains: []string{"app.example.com"},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "pool1"}, Weight: uint32Ptr(1)},
			},
			AdvertiseOnPublicDefaultVIP: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}
}

func TestBuildHTTPLoadBalancerCreate_RequiredFields(t *testing.T) {
	cr := sampleHTTPLoadBalancer("my-hlb", "default")

	result := buildHTTPLoadBalancerCreate(cr, "default")

	assert.Equal(t, "my-hlb", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, []string{"app.example.com"}, result.Spec.Domains)
	require.Len(t, result.Spec.DefaultRoutePools, 1)
	assert.Equal(t, "pool1", result.Spec.DefaultRoutePools[0].Pool.Name)
	assert.Equal(t, uint32(1), result.Spec.DefaultRoutePools[0].Weight)
}

func TestBuildHTTPLoadBalancerCreate_AppFirewallObjectRef(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-afw", "ns")
	cr.Spec.AppFirewall = &v1alpha1.ObjectRef{Name: "my-fw", Namespace: "shared"}

	result := buildHTTPLoadBalancerCreate(cr, "ns")

	require.NotNil(t, result.Spec.AppFirewall)
	assert.Equal(t, "my-fw", result.Spec.AppFirewall.Name)
	assert.Equal(t, "shared", result.Spec.AppFirewall.Namespace)
}

func TestBuildHTTPLoadBalancerCreate_TLSOneOf(t *testing.T) {
	httpsJSON := json.RawMessage(`{"tls_cert_params":{}}`)
	cr := sampleHTTPLoadBalancer("hlb-tls", "ns")
	cr.Spec.HTTPS = &apiextensionsv1.JSON{Raw: httpsJSON}

	result := buildHTTPLoadBalancerCreate(cr, "ns")

	assert.JSONEq(t, `{"tls_cert_params":{}}`, string(result.Spec.HTTPS))
	assert.Nil(t, result.Spec.HTTP)
	assert.Nil(t, result.Spec.HTTPSAutoCert)
}

func TestBuildHTTPLoadBalancerCreate_AdvertiseOneOf(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-adv", "ns")
	// sampleHTTPLoadBalancer sets AdvertiseOnPublicDefaultVIP already

	result := buildHTTPLoadBalancerCreate(cr, "ns")

	assert.JSONEq(t, `{}`, string(result.Spec.AdvertiseOnPublicDefaultVIP))
	assert.Nil(t, result.Spec.AdvertiseOnPublic)
	assert.Nil(t, result.Spec.AdvertiseCustom)
	assert.Nil(t, result.Spec.DoNotAdvertise)
}

func TestBuildHTTPLoadBalancerReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleHTTPLoadBalancer("my-hlb", "ns")

	result := buildHTTPLoadBalancerReplace(cr, "ns", "rv-42")

	assert.Equal(t, "my-hlb", result.Metadata.Name)
	assert.Equal(t, "ns", result.Metadata.Namespace)
	assert.Equal(t, "rv-42", result.Metadata.ResourceVersion)
	assert.Equal(t, []string{"app.example.com"}, result.Spec.Domains)
}

func TestBuildHTTPLoadBalancerDesiredSpecJSON(t *testing.T) {
	cr := sampleHTTPLoadBalancer("my-hlb", "ns")

	raw, err := buildHTTPLoadBalancerDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasDomains := spec["domains"]
	assert.True(t, hasDomains)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
