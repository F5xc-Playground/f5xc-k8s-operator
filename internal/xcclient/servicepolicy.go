package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ServicePolicySpec is the resource-specific payload for an F5 XC service
// policy. Rules holds the ordered list of policy rules as raw JSON so that
// callers can supply arbitrary rule structures without losing fidelity.
// Algo controls the match algorithm applied across the rule set.
type ServicePolicySpec struct {
	Rules []json.RawMessage `json:"rules,omitempty"`
	Algo  string            `json:"algo,omitempty"`
}

// ServicePolicyCreate is the request body for creating a new service policy.
type ServicePolicyCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

// ServicePolicyReplace is the request body for replacing an existing service
// policy.
type ServicePolicyReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     ServicePolicySpec `json:"spec"`
}

// ServicePolicy is the full service policy object returned by the F5 XC API.
// RawSpec holds the unparsed "spec" JSON from the server response and is
// excluded from marshalling (json:"-").
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
// Note: the F5 XC API uses the irregular plural "service_policys" as the
// resource path (ResourceServicePolicy = "service_policys").
func (c *Client) CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error) {
	sp.Metadata.Namespace = ns
	var result ServicePolicy
	if err := c.do(ctx, http.MethodPost, ResourceServicePolicy, ns, "", sp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetServicePolicy retrieves a service policy by name from the given namespace.
// The returned ServicePolicy has RawSpec populated with the raw "spec" JSON.
func (c *Client) GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceServicePolicy, ns, name, nil, &raw); err != nil {
		return nil, err
	}

	var result ServicePolicy
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshalling service policy: %w", err)
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
