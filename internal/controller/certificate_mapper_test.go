package controller

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testCertPEM = []byte("-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----")
	testKeyPEM  = []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----")
)

func sampleCertificate(name, namespace string) *v1alpha1.Certificate {
	return &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: namespace,
			SecretRef:   v1alpha1.SecretRef{Name: "my-tls-secret"},
		},
	}
}

func TestBuildCertificateCreate_BasicFields(t *testing.T) {
	cr := sampleCertificate("my-cert", "default")

	result := buildCertificateCreate(cr, "default", testCertPEM, testKeyPEM)
	assert.Equal(t, "my-cert", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)

	expectedCertURL := "string:///" + base64.StdEncoding.EncodeToString(testCertPEM)
	assert.Equal(t, expectedCertURL, result.Spec.CertificateURL)

	require.NotNil(t, result.Spec.PrivateKey.ClearSecretInfo)
	expectedKeyURL := "string:///" + base64.StdEncoding.EncodeToString(testKeyPEM)
	assert.Equal(t, expectedKeyURL, result.Spec.PrivateKey.ClearSecretInfo.URL)
	assert.Equal(t, "", result.Spec.PrivateKey.ClearSecretInfo.Provider)
}

func TestBuildCertificateCreate_OCSPFields(t *testing.T) {
	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-ocsp", Namespace: "ns"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace:         "ns",
			SecretRef:           v1alpha1.SecretRef{Name: "tls"},
			DisableOcspStapling: &apiextensionsv1.JSON{Raw: []byte(`{}`)},
		},
	}

	result := buildCertificateCreate(cr, "ns", testCertPEM, testKeyPEM)
	assert.JSONEq(t, `{}`, string(result.Spec.DisableOcspStapling))
	assert.Nil(t, result.Spec.CustomHashAlgorithms)
	assert.Nil(t, result.Spec.UseSystemDefaults)
}

func TestBuildCertificateCreate_NoOCSPFields(t *testing.T) {
	cr := sampleCertificate("cert-no-ocsp", "ns")

	result := buildCertificateCreate(cr, "ns", testCertPEM, testKeyPEM)
	assert.Nil(t, result.Spec.DisableOcspStapling)
	assert.Nil(t, result.Spec.CustomHashAlgorithms)
	assert.Nil(t, result.Spec.UseSystemDefaults)
}

func TestBuildCertificateReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleCertificate("my-cert", "ns")

	result := buildCertificateReplace(cr, "ns", "rv-5", testCertPEM, testKeyPEM)
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.NotEmpty(t, result.Spec.CertificateURL)
}

func TestBuildCertificateCreate_XCNamespaceOverride(t *testing.T) {
	cr := sampleCertificate("my-cert", "k8s-ns")

	result := buildCertificateCreate(cr, "xc-override", testCertPEM, testKeyPEM)
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildCertificateDesiredSpecJSON(t *testing.T) {
	cr := sampleCertificate("my-cert", "ns")

	raw, err := buildCertificateDesiredSpecJSON(cr, "ns", testCertPEM, testKeyPEM)
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasCertURL := spec["certificate_url"]
	assert.True(t, hasCertURL)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
