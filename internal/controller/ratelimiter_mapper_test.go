package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func uint32Ptr(v uint32) *uint32 { return &v }

func TestBuildRateLimiterCreate_BasicFields(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "default"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterCreate(cr, "default")
	assert.Equal(t, "my-rl", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, uint32(100), result.Spec.Threshold)
	assert.Equal(t, "MINUTE", result.Spec.Unit)
	assert.Equal(t, uint32(0), result.Spec.BurstMultiplier)
}

func TestBuildRateLimiterCreate_WithBurstMultiplier(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "burst-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold:       50,
			Unit:            "SECOND",
			BurstMultiplier: uint32Ptr(3),
		},
	}

	result := buildRateLimiterCreate(cr, "ns")
	assert.Equal(t, uint32(50), result.Spec.Threshold)
	assert.Equal(t, "SECOND", result.Spec.Unit)
	assert.Equal(t, uint32(3), result.Spec.BurstMultiplier)
}

func TestBuildRateLimiterReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.Equal(t, uint32(100), result.Spec.Threshold)
}

func TestBuildRateLimiterCreate_XCNamespaceOverride(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "k8s-ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildRateLimiterDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	raw, err := buildRateLimiterDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasTotalNumber := spec["total_number"]
	_, hasUnit := spec["unit"]
	assert.True(t, hasTotalNumber)
	assert.True(t, hasUnit)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
