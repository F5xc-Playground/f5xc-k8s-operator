package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// HealthCheck types
// ---------------------------------------------------------------------------

// HealthCheckSpec is the resource-specific payload for an F5 XC health check.
type HealthCheckSpec struct {
	HTTPHealthCheck    *HTTPHealthCheck `json:"http_health_check,omitempty"`
	TCPHealthCheck     *TCPHealthCheck  `json:"tcp_health_check,omitempty"`
	HealthyThreshold   uint32           `json:"healthy_threshold,omitempty"`
	UnhealthyThreshold uint32           `json:"unhealthy_threshold,omitempty"`
	Interval           uint32           `json:"interval,omitempty"`
	Timeout            uint32           `json:"timeout,omitempty"`
	JitterPercent      uint32           `json:"jitter_percent,omitempty"`
}

// HTTPHealthCheck configures an HTTP-based health check probe.
type HTTPHealthCheck struct {
	Path                string   `json:"path,omitempty"`
	UseHTTP2            bool     `json:"use_http2,omitempty"`
	ExpectedStatusCodes []string `json:"expected_status_codes,omitempty"`
}

// TCPHealthCheck configures a TCP-based health check probe.
type TCPHealthCheck struct {
	SendPayload      string `json:"send_payload,omitempty"`
	ExpectedResponse string `json:"expected_response,omitempty"`
}

// CreateHealthCheck is the request body for creating a new health check.
type CreateHealthCheck struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     HealthCheckSpec `json:"spec"`
}

// ReplaceHealthCheck is the request body for replacing an existing health check.
type ReplaceHealthCheck struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     HealthCheckSpec `json:"spec"`
}

// HealthCheck is the full health check object returned by the F5 XC API.
// RawSpec holds the unparsed "spec" JSON from the server response.
type HealthCheck struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	RawSpec        json.RawMessage `json:"spec"`
}

// ParseSpec decodes the raw spec JSON into a HealthCheckSpec.
func (h *HealthCheck) ParseSpec() (*HealthCheckSpec, error) {
	var spec HealthCheckSpec
	if err := json.Unmarshal(h.RawSpec, &spec); err != nil {
		return nil, fmt.Errorf("unmarshalling health check spec: %w", err)
	}
	return &spec, nil
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateHealthCheck creates a new health check in the given namespace.
func (c *Client) CreateHealthCheck(ctx context.Context, ns string, hc CreateHealthCheck) (*HealthCheck, error) {
	hc.Metadata.Namespace = ns
	var result HealthCheck
	if err := c.do(ctx, http.MethodPost, ResourceHealthCheck, ns, "", hc, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHealthCheck retrieves a health check by name from the given namespace.
func (c *Client) GetHealthCheck(ctx context.Context, ns, name string) (*HealthCheck, error) {
	var result HealthCheck
	if err := c.do(ctx, http.MethodGet, ResourceHealthCheck, ns, name, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReplaceHealthCheck replaces an existing health check identified by name.
func (c *Client) ReplaceHealthCheck(ctx context.Context, ns, name string, hc ReplaceHealthCheck) (*HealthCheck, error) {
	hc.Metadata.Namespace = ns
	hc.Metadata.Name = name
	var result HealthCheck
	if err := c.do(ctx, http.MethodPut, ResourceHealthCheck, ns, name, hc, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteHealthCheck removes the health check identified by name from the given
// namespace.
func (c *Client) DeleteHealthCheck(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceHealthCheck, ns, name, nil, nil)
}

// ListHealthChecks returns all health checks in the given namespace.
func (c *Client) ListHealthChecks(ctx context.Context, ns string) ([]*HealthCheck, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceHealthCheck, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[HealthCheck](raw)
}
