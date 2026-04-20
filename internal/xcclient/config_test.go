package xcclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_RequiresTenantURL(t *testing.T) {
	cfg := Config{
		APIToken: "tok",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TenantURL")
}

func TestConfig_Validate_RequiresAuth_NeitherSet(t *testing.T) {
	cfg := Config{
		TenantURL: "https://example.console.ves.volterra.io",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth")
}

func TestConfig_Validate_RejectsBothAuthMethods(t *testing.T) {
	cfg := Config{
		TenantURL:   "https://example.console.ves.volterra.io",
		APIToken:    "tok",
		CertP12Path: "/path/to/cert.p12",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestConfig_Validate_AcceptsAPITokenOnly(t *testing.T) {
	cfg := Config{
		TenantURL: "https://example.console.ves.volterra.io",
		APIToken:  "tok",
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_AcceptsP12Only(t *testing.T) {
	cfg := Config{
		TenantURL:   "https://example.console.ves.volterra.io",
		CertP12Path: "/path/to/cert.p12",
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_StripsTrailingSlash(t *testing.T) {
	cfg := Config{
		TenantURL: "https://example.console.ves.volterra.io/",
		APIToken:  "tok",
	}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "https://example.console.ves.volterra.io", cfg.TenantURL)
}

func TestConfig_Validate_SetsAllDefaults(t *testing.T) {
	cfg := Config{
		TenantURL: "https://example.console.ves.volterra.io",
		APIToken:  "tok",
	}
	err := cfg.Validate()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.HTTPTimeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, float64(2), cfg.RateLimits.DefaultRPS)
	assert.Equal(t, 5, cfg.RateLimits.DefaultBurst)
}
