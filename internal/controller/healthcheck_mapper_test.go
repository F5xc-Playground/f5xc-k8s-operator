package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildHealthCheckCreate_HTTPProbe(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "default"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{
				Path:                "/healthz",
				UseHTTP2:            true,
				ExpectedStatusCodes: []string{"200", "201"},
			},
			Interval:           uint32Ptr(30),
			Timeout:            uint32Ptr(5),
			HealthyThreshold:   uint32Ptr(3),
			UnhealthyThreshold: uint32Ptr(2),
			JitterPercent:      uint32Ptr(10),
		},
	}

	result := buildHealthCheckCreate(cr, "default")
	assert.Equal(t, "my-hc", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	require.NotNil(t, result.Spec.HTTPHealthCheck)
	assert.Equal(t, "/healthz", result.Spec.HTTPHealthCheck.Path)
	assert.True(t, result.Spec.HTTPHealthCheck.UseHTTP2)
	assert.Equal(t, []string{"200", "201"}, result.Spec.HTTPHealthCheck.ExpectedStatusCodes)
	assert.Equal(t, uint32(30), result.Spec.Interval)
	assert.Equal(t, uint32(5), result.Spec.Timeout)
	assert.Equal(t, uint32(3), result.Spec.HealthyThreshold)
	assert.Equal(t, uint32(2), result.Spec.UnhealthyThreshold)
	assert.Equal(t, uint32(10), result.Spec.JitterPercent)
}

func TestBuildHealthCheckCreate_TCPProbe(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			TCPHealthCheck: &v1alpha1.TCPHealthCheckSpec{
				Send:    "PING",
				Receive: "PONG",
			},
		},
	}

	result := buildHealthCheckCreate(cr, "ns")
	require.NotNil(t, result.Spec.TCPHealthCheck)
	assert.Equal(t, "PING", result.Spec.TCPHealthCheck.Send)
	assert.Equal(t, "PONG", result.Spec.TCPHealthCheck.Receive)
	assert.Nil(t, result.Spec.HTTPHealthCheck)
}

func TestBuildHealthCheckCreate_OptionalFieldsOmitted(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "minimal-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/"},
		},
	}

	result := buildHealthCheckCreate(cr, "ns")
	assert.Equal(t, uint32(0), result.Spec.Interval)
	assert.Equal(t, uint32(0), result.Spec.Timeout)
	assert.Equal(t, uint32(0), result.Spec.HealthyThreshold)
	assert.Equal(t, uint32(0), result.Spec.UnhealthyThreshold)
}

func TestBuildHealthCheckReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/healthz"},
		},
	}

	result := buildHealthCheckReplace(cr, "ns", "rv-3")
	assert.Equal(t, "rv-3", result.Metadata.ResourceVersion)
}

func TestBuildHealthCheckDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/healthz"},
			Interval:        uint32Ptr(30),
		},
	}

	raw, err := buildHealthCheckDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasHTTPHealthCheck := spec["http_health_check"]
	_, hasInterval := spec["interval"]
	assert.True(t, hasHTTPHealthCheck)
	assert.True(t, hasInterval)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
