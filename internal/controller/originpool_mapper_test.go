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

func TestBuildOriginPoolCreate_BasicFields(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pool",
			Namespace: "default",
		},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			Port:                  443,
			LoadBalancerAlgorithm: "ROUND_ROBIN",
		},
	}

	result := buildOriginPoolCreate(cr, "default")
	assert.Equal(t, "my-pool", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, 443, result.Spec.Port)
	assert.Equal(t, "ROUND_ROBIN", result.Spec.LoadBalancerAlgorithm)
	require.Len(t, result.Spec.OriginServers, 1)
	assert.Equal(t, "1.2.3.4", result.Spec.OriginServers[0].PublicIP.IP)
}

func TestBuildOriginPoolCreate_AllOriginServerTypes(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "all-types", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.1.1.1"}},
				{PublicName: &v1alpha1.PublicName{DNSName: "example.com"}},
				{PrivateIP: &v1alpha1.PrivateIP{IP: "10.0.0.1", Site: &v1alpha1.ObjectRef{Name: "site1"}}},
				{PrivateName: &v1alpha1.PrivateName{DNSName: "internal.local", Site: &v1alpha1.ObjectRef{Name: "site2"}}},
				{K8SService: &v1alpha1.K8SService{ServiceName: "my-svc", ServiceNamespace: "kube-system", Site: &v1alpha1.ObjectRef{Name: "site3"}}},
				{ConsulService: &v1alpha1.ConsulService{ServiceName: "consul-svc", Site: &v1alpha1.ObjectRef{Name: "site4"}}},
			},
			Port: 80,
		},
	}

	result := buildOriginPoolCreate(cr, "ns")
	require.Len(t, result.Spec.OriginServers, 6)

	assert.Equal(t, "1.1.1.1", result.Spec.OriginServers[0].PublicIP.IP)
	assert.Equal(t, "example.com", result.Spec.OriginServers[1].PublicName.DNSName)
	assert.Equal(t, "10.0.0.1", result.Spec.OriginServers[2].PrivateIP.IP)
	assert.Equal(t, "site1", result.Spec.OriginServers[2].PrivateIP.Site.Name)
	assert.Equal(t, "internal.local", result.Spec.OriginServers[3].PrivateName.DNSName)
	assert.Equal(t, "my-svc", result.Spec.OriginServers[4].K8SService.ServiceName)
	assert.Equal(t, "kube-system", result.Spec.OriginServers[4].K8SService.ServiceNamespace)
	assert.Equal(t, "consul-svc", result.Spec.OriginServers[5].ConsulService.ServiceName)
}

func TestBuildOriginPoolCreate_HealthChecks(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "hc-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 80,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			HealthChecks: []v1alpha1.ObjectRef{
				{Name: "hc1", Namespace: "ns"},
			},
		},
	}

	result := buildOriginPoolCreate(cr, "ns")
	require.Len(t, result.Spec.HealthCheck, 1)
	assert.Equal(t, "hc1", result.Spec.HealthCheck[0].Name)
	assert.Equal(t, "ns", result.Spec.HealthCheck[0].Namespace)
}

func TestBuildOriginPoolCreate_TLSPassthrough(t *testing.T) {
	tlsJSON := json.RawMessage(`{"tls_config":{"default_security":{}}}`)
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			UseTLS: &apiextensionsv1.JSON{Raw: tlsJSON},
		},
	}

	result := buildOriginPoolCreate(cr, "ns")
	assert.JSONEq(t, `{"tls_config":{"default_security":{}}}`, string(result.Spec.UseTLS))
}

func TestBuildOriginPoolReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
		},
	}

	result := buildOriginPoolReplace(cr, "ns", "rv-123")
	assert.Equal(t, "my-pool", result.Metadata.Name)
	assert.Equal(t, "ns", result.Metadata.Namespace)
	assert.Equal(t, "rv-123", result.Metadata.ResourceVersion)
	assert.Equal(t, 443, result.Spec.Port)
}

func TestBuildOriginPoolCreate_XCNamespaceOverride(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pool", Namespace: "k8s-ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 80,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
		},
	}

	result := buildOriginPoolCreate(cr, "xc-override-ns")
	assert.Equal(t, "xc-override-ns", result.Metadata.Namespace)
}

func TestBuildDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
		},
	}

	raw, err := buildOriginPoolDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	// buildDesiredSpecJSON returns the spec JSON only (same format as RawSpec
	// from the server) so that it can be compared directly with current.RawSpec
	// in ClientNeedsUpdate.
	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasOriginServers := spec["origin_servers"]
	_, hasPort := spec["port"]
	assert.True(t, hasOriginServers)
	assert.True(t, hasPort)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
