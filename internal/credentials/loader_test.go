package credentials_test

import (
	"os"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/credentials"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigFromSecret_APIToken(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds"},
		Data: map[string][]byte{
			"api-token": []byte("my-token"),
		},
	}
	cfg, err := credentials.ConfigFromSecret(secret, "https://tenant.console.ves.volterra.io", xcclient.RateLimitConfig{})
	require.NoError(t, err)
	assert.Equal(t, "my-token", cfg.APIToken)
	assert.Equal(t, "https://tenant.console.ves.volterra.io", cfg.TenantURL)
	assert.Empty(t, cfg.CertP12Path)
}

func TestConfigFromSecret_P12(t *testing.T) {
	p12Bytes := []byte("fake-p12-data")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds"},
		Data: map[string][]byte{
			"cert.p12":      p12Bytes,
			"cert-password": []byte("p12pass"),
		},
	}
	cfg, err := credentials.ConfigFromSecret(secret, "https://tenant.console.ves.volterra.io", xcclient.RateLimitConfig{})
	require.NoError(t, err)
	assert.Empty(t, cfg.APIToken)
	assert.NotEmpty(t, cfg.CertP12Path)
	assert.Equal(t, "p12pass", cfg.CertPassword)

	// Verify the temp file was written with the P12 bytes.
	data, err := os.ReadFile(cfg.CertP12Path)
	require.NoError(t, err)
	assert.Equal(t, p12Bytes, data)

	// Cleanup temp file.
	os.Remove(cfg.CertP12Path)
}

func TestConfigFromSecret_BothAuthMethods(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds"},
		Data: map[string][]byte{
			"api-token":     []byte("my-token"),
			"cert.p12":      []byte("p12data"),
			"cert-password": []byte("pass"),
		},
	}
	_, err := credentials.ConfigFromSecret(secret, "https://tenant.console.ves.volterra.io", xcclient.RateLimitConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestConfigFromSecret_NeitherAuthMethod(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds"},
		Data:       map[string][]byte{},
	}
	_, err := credentials.ConfigFromSecret(secret, "https://tenant.console.ves.volterra.io", xcclient.RateLimitConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain")
}

func TestConfigFromSecret_P12MissingPassword(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds"},
		Data: map[string][]byte{
			"cert.p12": []byte("p12data"),
		},
	}
	_, err := credentials.ConfigFromSecret(secret, "https://tenant.console.ves.volterra.io", xcclient.RateLimitConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cert-password")
}
