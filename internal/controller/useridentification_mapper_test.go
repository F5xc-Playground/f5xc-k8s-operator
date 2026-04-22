package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleUserIdentification(name, namespace string) *v1alpha1.UserIdentification {
	return &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: namespace,
			Rules: []v1alpha1.UserIdentificationRule{
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
}

func TestBuildUserIdentificationCreate_BasicFields(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "default")
	result := buildUserIdentificationCreate(cr, "default")
	assert.Equal(t, "my-uid", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildUserIdentificationCreate_ClientIPRule(t *testing.T) {
	cr := sampleUserIdentification("uid-ip", "ns")
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"client_ip":{}`)
}

func TestBuildUserIdentificationCreate_CookieNameRule(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-cookie", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{CookieName: "Session"},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"cookie_name":"Session"`)
}

func TestBuildUserIdentificationCreate_HTTPHeaderRule(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-header", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{HTTPHeaderName: "Authorization"},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"http_header_name":"Authorization"`)
}

func TestBuildUserIdentificationCreate_MultipleRules(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-multi", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{JWTClaimName: "sub"},
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"jwt_claim_name":"sub"`)
	assert.Contains(t, string(raw), `"client_ip":{}`)
}

func TestBuildUserIdentificationCreate_TLSFingerprint(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-tls", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{IPAndTLSFingerprint: &v1alpha1.EmptyObject{}},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"ip_and_tls_fingerprint":{}`)
}

func TestBuildUserIdentificationReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "ns")
	result := buildUserIdentificationReplace(cr, "ns", "rv-7")
	assert.Equal(t, "rv-7", result.Metadata.ResourceVersion)
}

func TestBuildUserIdentificationDesiredSpecJSON(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "ns")
	raw, err := buildUserIdentificationDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasRules := spec["rules"]
	assert.True(t, hasRules)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
