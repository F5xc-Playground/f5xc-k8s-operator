package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sampleTCPLoadBalancer(name, namespace string) *v1alpha1.TCPLoadBalancer {
	return &v1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.TCPLoadBalancerSpec{
			XCNamespace: namespace,
			Domains:     []string{"tcp.example.com"},
			ListenPort:  443,
			OriginPools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "pool1"}, Weight: uint32Ptr(1)},
			},
		},
	}
}

func TestBuildTCPLoadBalancerCreate_BasicFields(t *testing.T) {
	cr := sampleTCPLoadBalancer("my-tlb", "default")

	result := buildTCPLoadBalancerCreate(cr, "default")

	assert.Equal(t, "my-tlb", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, []string{"tcp.example.com"}, result.Spec.Domains)
	assert.Equal(t, uint32(443), result.Spec.ListenPort)
	require.Len(t, result.Spec.OriginPoolWeights, 1)
	assert.Equal(t, "pool1", result.Spec.OriginPoolWeights[0].Pool.Name)
	assert.Equal(t, uint32(1), result.Spec.OriginPoolWeights[0].Weight)
}

func TestBuildTCPLoadBalancerCreate_RoutePoolWeightPriority(t *testing.T) {
	cr := &v1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "tlb-wp", Namespace: "ns"},
		Spec: v1alpha1.TCPLoadBalancerSpec{
			Domains:    []string{"wp.example.com"},
			ListenPort: 8080,
			OriginPools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "pool-a", Namespace: "shared"}, Weight: uint32Ptr(10), Priority: uint32Ptr(2)},
				{Pool: v1alpha1.ObjectRef{Name: "pool-b"}, Weight: nil, Priority: nil},
			},
		},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")

	require.Len(t, result.Spec.OriginPoolWeights, 2)
	assert.Equal(t, "pool-a", result.Spec.OriginPoolWeights[0].Pool.Name)
	assert.Equal(t, "shared", result.Spec.OriginPoolWeights[0].Pool.Namespace)
	assert.Equal(t, uint32(10), result.Spec.OriginPoolWeights[0].Weight)
	assert.Equal(t, uint32(2), result.Spec.OriginPoolWeights[0].Priority)

	assert.Equal(t, "pool-b", result.Spec.OriginPoolWeights[1].Pool.Name)
	assert.Equal(t, uint32(0), result.Spec.OriginPoolWeights[1].Weight)
	assert.Equal(t, uint32(0), result.Spec.OriginPoolWeights[1].Priority)
}

func TestBuildTCPLoadBalancerCreate_TLSTCPAutoCert(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-tls", "ns")
	cr.Spec.TLSTCPAutoCert = &v1alpha1.TLSTCPAutoCert{
		NoMTLS: &v1alpha1.EmptyObject{},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"no_mtls":{}}`, string(result.Spec.TLSTCPAutoCert))
	assert.Nil(t, result.Spec.TCP)
	assert.Nil(t, result.Spec.TLSTCP)
}

func TestBuildTCPLoadBalancerCreate_NoTLS(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-notls", "ns")
	cr.Spec.NoTLS = &v1alpha1.EmptyObject{}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.TCP))
	assert.Nil(t, result.Spec.TLSTCP)
}

func TestBuildTCPLoadBalancerCreate_TLSParameters(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-tls-params", "ns")
	cr.Spec.TLSParameters = &v1alpha1.TLSParameters{
		DefaultSecurity: &v1alpha1.EmptyObject{},
		NoMTLS:          &v1alpha1.EmptyObject{},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.TLSTCP)
	var tls map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.TLSTCP, &tls))
	_, hasDefaultSecurity := tls["default_security"]
	assert.True(t, hasDefaultSecurity)
}

func TestBuildTCPLoadBalancerCreate_AdvertiseCustom(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-adv", "ns")
	cr.Spec.AdvertiseCustom = &v1alpha1.AdvertiseCustom{
		AdvertiseWhere: []v1alpha1.AdvertiseWhere{
			{Port: 8080, Site: &v1alpha1.AdvertiseSite{
				Network: "SITE_NETWORK_INSIDE",
				Site:    v1alpha1.ObjectRef{Name: "my-site"},
			}},
		},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.AdvertiseCustom)
}

func TestBuildTCPLoadBalancerReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleTCPLoadBalancer("my-tlb", "ns")

	result := buildTCPLoadBalancerReplace(cr, "ns", "rv-999")

	assert.Equal(t, "my-tlb", result.Metadata.Name)
	assert.Equal(t, "ns", result.Metadata.Namespace)
	assert.Equal(t, "rv-999", result.Metadata.ResourceVersion)
	assert.Equal(t, uint32(443), result.Spec.ListenPort)
}

func TestBuildTCPLoadBalancerCreate_XCNamespaceOverride(t *testing.T) {
	cr := sampleTCPLoadBalancer("my-tlb", "k8s-ns")

	result := buildTCPLoadBalancerCreate(cr, "xc-override-ns")

	assert.Equal(t, "xc-override-ns", result.Metadata.Namespace)
}

func TestBuildTCPLoadBalancerDesiredSpecJSON(t *testing.T) {
	cr := sampleTCPLoadBalancer("my-tlb", "ns")

	raw, err := buildTCPLoadBalancerDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasDomains := spec["domains"]
	_, hasListenPort := spec["listen_port"]
	assert.True(t, hasDomains)
	assert.True(t, hasListenPort)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
