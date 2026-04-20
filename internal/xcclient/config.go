package xcclient

import (
	"fmt"
	"strings"
	"time"
)

// Config holds all configuration for the XC API client.
type Config struct {
	TenantURL    string
	APIToken     string
	CertP12Path  string
	CertPassword string
	RateLimits   RateLimitConfig
	HTTPTimeout  time.Duration
	MaxRetries   int
}

// RateLimitConfig defines global and per-endpoint rate limit settings.
type RateLimitConfig struct {
	DefaultRPS   float64
	DefaultBurst int
	Overrides    map[string]EndpointLimit
}

// EndpointLimit defines per-endpoint rate limit overrides.
type EndpointLimit struct {
	RPS   float64
	Burst int
}

// Validate checks that the Config is well-formed, strips trailing slashes from
// TenantURL, and applies default values for any unset fields.
func (c *Config) Validate() error {
	if c.TenantURL == "" {
		return fmt.Errorf("TenantURL is required")
	}

	hasToken := c.APIToken != ""
	hasCert := c.CertP12Path != ""

	if !hasToken && !hasCert {
		return fmt.Errorf("auth credentials required: set either APIToken or CertP12Path")
	}
	if hasToken && hasCert {
		return fmt.Errorf("APIToken and CertP12Path are mutually exclusive: provide only one auth method")
	}

	// Normalize TenantURL.
	c.TenantURL = strings.TrimRight(c.TenantURL, "/")

	// Apply defaults.
	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = 30 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 5
	}
	if c.RateLimits.DefaultRPS == 0 {
		c.RateLimits.DefaultRPS = 2
	}
	if c.RateLimits.DefaultBurst == 0 {
		c.RateLimits.DefaultBurst = 5
	}

	return nil
}
