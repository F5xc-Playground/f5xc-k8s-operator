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
	require.Len(t, result.Spec.OriginPools, 1)
	assert.Equal(t, "pool1", result.Spec.OriginPools[0].Pool.Name)
	assert.Equal(t, uint32(1), result.Spec.OriginPools[0].Weight)
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

	require.Len(t, result.Spec.OriginPools, 2)
	assert.Equal(t, "pool-a", result.Spec.OriginPools[0].Pool.Name)
	assert.Equal(t, "shared", result.Spec.OriginPools[0].Pool.Namespace)
	assert.Equal(t, uint32(10), result.Spec.OriginPools[0].Weight)
	assert.Equal(t, uint32(2), result.Spec.OriginPools[0].Priority)

	assert.Equal(t, "pool-b", result.Spec.OriginPools[1].Pool.Name)
	assert.Equal(t, uint32(0), result.Spec.OriginPools[1].Weight)
	assert.Equal(t, uint32(0), result.Spec.OriginPools[1].Priority)
}

func TestBuildTCPLoadBalancerCreate_TLSPassthrough(t *testing.T) {
	tlsJSON := json.RawMessage(`{"sni_check":{}}`)
	cr := sampleTCPLoadBalancer("tlb-tls", "ns")
	cr.Spec.TLSPassthrough = &apiextensionsv1.JSON{Raw: tlsJSON}

	result := buildTCPLoadBalancerCreate(cr, "ns")

	assert.JSONEq(t, `{"sni_check":{}}`, string(result.Spec.TLSPassthrough))
	assert.Nil(t, result.Spec.NoTLS)
	assert.Nil(t, result.Spec.TLSParameters)
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
