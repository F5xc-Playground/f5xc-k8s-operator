package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sampleHTTPLoadBalancer(name, namespace string) *v1alpha1.HTTPLoadBalancer {
	return &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: namespace,
			Domains:     []string{"app.example.com"},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "pool1"}, Weight: uint32Ptr(1)},
			},
			AdvertiseOnPublicDefaultVIP: &v1alpha1.EmptyObject{},
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
	cr := sampleHTTPLoadBalancer("hlb-tls", "ns")
	cr.Spec.HTTPS = &v1alpha1.HTTPSConfig{
		HTTPRedirect:    true,
		AddHSTS:         true,
		DefaultSecurity: &v1alpha1.EmptyObject{},
		NoMTLS:          &v1alpha1.EmptyObject{},
		Port:            443,
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.HTTPS)
	var https map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.HTTPS, &https))
	assert.Equal(t, true, https["http_redirect"])
	assert.Equal(t, true, https["add_hsts"])
}

func TestBuildHTTPLoadBalancerCreate_AdvertiseOneOf(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-adv", "ns")
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.AdvertiseOnPublicDefaultVIP))
	assert.Nil(t, result.Spec.AdvertiseOnPublic)
	assert.Nil(t, result.Spec.AdvertiseCustom)
	assert.Nil(t, result.Spec.DoNotAdvertise)
}

func TestBuildHTTPLoadBalancerCreate_HTTP(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-http", "ns")
	cr.Spec.HTTP = &v1alpha1.HTTPConfig{DNSVolterraManaged: false, Port: 80}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"port":80}`, string(result.Spec.HTTP))
}

func TestBuildHTTPLoadBalancerCreate_CookieStickiness(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-cookie", "ns")
	cr.Spec.CookieStickiness = &v1alpha1.CookieStickinessConfig{Name: "session", Path: "/", TTL: 3600}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"name":"session","path":"/","ttl":3600}`, string(result.Spec.CookieStickiness))
}

func TestBuildHTTPLoadBalancerCreate_DisableOptions(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-disabled", "ns")
	cr.Spec.DisableWAF = &v1alpha1.EmptyObject{}
	cr.Spec.DisableBotDefense = &v1alpha1.EmptyObject{}
	cr.Spec.DisableAPIDiscovery = &v1alpha1.EmptyObject{}
	cr.Spec.NoChallenge = &v1alpha1.EmptyObject{}
	cr.Spec.RoundRobin = &v1alpha1.EmptyObject{}
	cr.Spec.NoServicePolicies = &v1alpha1.EmptyObject{}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.DisableWAF))
	assert.JSONEq(t, `{}`, string(result.Spec.DisableBotDefense))
	assert.JSONEq(t, `{}`, string(result.Spec.DisableAPIDiscovery))
	assert.JSONEq(t, `{}`, string(result.Spec.NoChallenge))
	assert.JSONEq(t, `{}`, string(result.Spec.RoundRobin))
	assert.JSONEq(t, `{}`, string(result.Spec.NoServicePolicies))
}

func TestBuildHTTPLoadBalancerCreate_ActiveServicePolicies(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-sp", "ns")
	cr.Spec.ActiveServicePolicies = &v1alpha1.ActiveServicePoliciesConfig{
		Policies: []v1alpha1.ObjectRef{{Name: "policy1", Namespace: "ns"}},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"policies":[{"name":"policy1","namespace":"ns"}]}`, string(result.Spec.ActiveServicePolicies))
}

func TestBuildHTTPLoadBalancerCreate_UserIDClientIP(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-uid", "ns")
	cr.Spec.UserIDClientIP = &v1alpha1.EmptyObject{}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.UserIDClientIP))
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
