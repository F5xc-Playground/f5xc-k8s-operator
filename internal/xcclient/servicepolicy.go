package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// ServicePolicySpec is the resource-specific payload for an F5 XC service policy.
type ServicePolicySpec struct {
	// Rule choice OneOf: allow_all_requests, allow_list, deny_all_requests, deny_list, rule_list
	AllowAllRequests json.RawMessage `json:"allow_all_requests,omitempty"`
	AllowList        json.RawMessage `json:"allow_list,omitempty"`
	DenyAllRequests  json.RawMessage `json:"deny_all_requests,omitempty"`
	DenyList         json.RawMessage `json:"deny_list,omitempty"`
	RuleList         json.RawMessage `json:"rule_list,omitempty"`

	// Server choice OneOf: any_server, server_name, server_name_matcher, server_selector
	AnyServer         json.RawMessage `json:"any_server,omitempty"`
	ServerName        string          `json:"server_name,omitempty"`
	ServerNameMatcher json.RawMessage `json:"server_name_matcher,omitempty"`
	ServerSelector    json.RawMessage `json:"server_selector,omitempty"`
}

// ServicePolicyCreate is the request body for creating a new service policy.
type ServicePolicyCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

// ServicePolicyReplace is the request body for replacing an existing service policy.
type ServicePolicyReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

// ServicePolicy is the full service policy object returned by the F5 XC API.
// RawSpec holds the unparsed "spec" JSON from the server response.
type ServicePolicy struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           ServicePolicySpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateServicePolicy creates a new service policy in the given namespace.
func (c *Client) CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error) {
	sp.Metadata.Namespace = ns
	var result ServicePolicy
	if err := c.do(ctx, http.MethodPost, ResourceServicePolicy, ns, "", sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetServicePolicy retrieves a service policy by name from the given namespace.
func (c *Client) GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceServicePolicy, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result ServicePolicy
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceServicePolicy replaces an existing service policy identified by name.
func (c *Client) ReplaceServicePolicy(ctx context.Context, ns, name string, sp *ServicePolicyReplace) (*ServicePolicy, error) {
	sp.Metadata.Namespace = ns
	sp.Metadata.Name = name
	var result ServicePolicy
	if err := c.do(ctx, http.MethodPut, ResourceServicePolicy, ns, name, sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteServicePolicy removes the service policy identified by name from the
// given namespace.
func (c *Client) DeleteServicePolicy(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceServicePolicy, ns, name, nil, nil)
}

// ListServicePolicies returns all service policies in the given namespace.
func (c *Client) ListServicePolicies(ctx context.Context, ns string) ([]*ServicePolicy, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceServicePolicy, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[ServicePolicy](raw)
}
