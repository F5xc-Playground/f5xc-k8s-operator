package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleMaliciousUserMitigation(name, namespace string) *v1alpha1.MaliciousUserMitigation {
	return &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: namespace,
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
}

func TestBuildMaliciousUserMitigationCreate_BasicFields(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "default")
	result := buildMaliciousUserMitigationCreate(cr, "default")
	assert.Equal(t, "my-mum", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildMaliciousUserMitigationCreate_AllThreatLevels(t *testing.T) {
	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: "mum-all", Namespace: "ns"},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: "ns",
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Medium: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{CaptchaChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{High: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{BlockTemporarily: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
	result := buildMaliciousUserMitigationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	s := string(raw)
	assert.Contains(t, s, `"low":{}`)
	assert.Contains(t, s, `"medium":{}`)
	assert.Contains(t, s, `"high":{}`)
	assert.Contains(t, s, `"javascript_challenge":{}`)
	assert.Contains(t, s, `"captcha_challenge":{}`)
	assert.Contains(t, s, `"block_temporarily":{}`)
}

func TestBuildMaliciousUserMitigationCreate_NilMitigationType(t *testing.T) {
	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: "mum-nil", Namespace: "ns"},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: "ns",
		},
	}
	result := buildMaliciousUserMitigationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "mitigation_type")
}

func TestBuildMaliciousUserMitigationReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "ns")
	result := buildMaliciousUserMitigationReplace(cr, "ns", "rv-2")
	assert.Equal(t, "rv-2", result.Metadata.ResourceVersion)
}

func TestBuildMaliciousUserMitigationDesiredSpecJSON(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "ns")
	raw, err := buildMaliciousUserMitigationDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasMT := spec["mitigation_type"]
	assert.True(t, hasMT)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
