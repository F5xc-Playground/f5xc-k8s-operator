package credentials

import (
	"fmt"
	"os"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	corev1 "k8s.io/api/core/v1"
)

func ConfigFromSecret(secret *corev1.Secret, tenantURL string, rateLimits xcclient.RateLimitConfig) (xcclient.Config, error) {
	apiToken := string(secret.Data["api-token"])
	p12Data := secret.Data["cert.p12"]
	certPassword := string(secret.Data["cert-password"])

	hasToken := apiToken != ""
	hasP12 := len(p12Data) > 0

	if hasToken && hasP12 {
		return xcclient.Config{}, fmt.Errorf("secret %s/%s contains both api-token and cert.p12: mutually exclusive", secret.Namespace, secret.Name)
	}
	if !hasToken && !hasP12 {
		return xcclient.Config{}, fmt.Errorf("secret %s/%s must contain either api-token or cert.p12", secret.Namespace, secret.Name)
	}

	cfg := xcclient.Config{
		TenantURL:  tenantURL,
		RateLimits: rateLimits,
	}

	if hasToken {
		cfg.APIToken = apiToken
		return cfg, nil
	}

	if certPassword == "" {
		return xcclient.Config{}, fmt.Errorf("secret %s/%s contains cert.p12 but missing cert-password", secret.Namespace, secret.Name)
	}

	tmpFile, err := os.CreateTemp("", "xc-p12-*.p12")
	if err != nil {
		return xcclient.Config{}, fmt.Errorf("creating temp file for P12: %w", err)
	}
	if _, err := tmpFile.Write(p12Data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return xcclient.Config{}, fmt.Errorf("writing P12 to temp file: %w", err)
	}
	tmpFile.Close()

	cfg.CertP12Path = tmpFile.Name()
	cfg.CertPassword = certPassword
	return cfg, nil
}
