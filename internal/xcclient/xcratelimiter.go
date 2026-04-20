package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// XCRateLimiterSpec is the resource-specific payload for an F5 XC rate limiter.
// This configures traffic rate limiting in XC (not our internal token bucket).
type XCRateLimiterSpec struct {
	Threshold       uint32 `json:"total_number,omitempty"`
	Unit            string `json:"unit,omitempty"`
	BurstMultiplier uint32 `json:"burst_multiplier,omitempty"`
}

// XCRateLimiterCreate is the request body for creating a new XC rate limiter.
type XCRateLimiterCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     XCRateLimiterSpec `json:"spec"`
}

// XCRateLimiterReplace is the request body for replacing an existing XC rate limiter.
type XCRateLimiterReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     XCRateLimiterSpec `json:"spec"`
}

// XCRateLimiter is the full rate limiter object returned by the F5 XC API.
// RawSpec holds the unparsed "spec" JSON from the server response and is
// excluded from marshalling (json:"-").
type XCRateLimiter struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           XCRateLimiterSpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateRateLimiter creates a new XC rate limiter in the given namespace.
func (c *Client) CreateRateLimiter(ctx context.Context, ns string, rl XCRateLimiterCreate) (*XCRateLimiter, error) {
	var result XCRateLimiter
	if err := c.do(ctx, http.MethodPost, ResourceRateLimiter, ns, "", rl, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRateLimiter retrieves an XC rate limiter by name from the given namespace.
// The returned XCRateLimiter has RawSpec populated with the raw "spec" JSON.
func (c *Client) GetRateLimiter(ctx context.Context, ns, name string) (*XCRateLimiter, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceRateLimiter, ns, name, nil, &raw); err != nil {
		return nil, err
	}

	var result XCRateLimiter
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshalling rate limiter: %w", err)
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceRateLimiter replaces an existing XC rate limiter identified by name.
func (c *Client) ReplaceRateLimiter(ctx context.Context, ns, name string, rl XCRateLimiterReplace) (*XCRateLimiter, error) {
	var result XCRateLimiter
	if err := c.do(ctx, http.MethodPut, ResourceRateLimiter, ns, name, rl, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteRateLimiter removes the XC rate limiter identified by name from the
// given namespace.
func (c *Client) DeleteRateLimiter(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceRateLimiter, ns, name, nil, nil)
}

// ListRateLimiters returns all XC rate limiters in the given namespace.
func (c *Client) ListRateLimiters(ctx context.Context, ns string) ([]*XCRateLimiter, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceRateLimiter, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[XCRateLimiter](raw)
}
