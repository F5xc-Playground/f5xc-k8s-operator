package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// HTTPLoadBalancerSpec is the resource-specific payload for an F5 XC HTTP
// load balancer.
type HTTPLoadBalancerSpec struct {
	Domains           []string    `json:"domains,omitempty"`
	DefaultRoutePools []RoutePool `json:"default_route_pools,omitempty"`
	AppFirewall       *ObjectRef  `json:"app_firewall,omitempty"`
	RateLimiter       *ObjectRef  `json:"rate_limiter,omitempty"`

	// Protocol — OneOf: http, https, https_auto_cert
	HTTP          json.RawMessage `json:"http,omitempty"`
	HTTPS         json.RawMessage `json:"https,omitempty"`
	HTTPSAutoCert json.RawMessage `json:"https_auto_cert,omitempty"`

	// Load balancing algorithm — OneOf: round_robin, least_active, random, ring_hash, etc.
	RoundRobin  json.RawMessage `json:"round_robin,omitempty"`
	LeastActive json.RawMessage `json:"least_active,omitempty"`
	Random      json.RawMessage `json:"random,omitempty"`
	RingHash    json.RawMessage `json:"ring_hash,omitempty"`

	// Advertise — OneOf
	AdvertiseOnPublicDefaultVIP json.RawMessage `json:"advertise_on_public_default_vip,omitempty"`
	AdvertiseOnPublic           json.RawMessage `json:"advertise_on_public,omitempty"`
	AdvertiseCustom             json.RawMessage `json:"advertise_custom,omitempty"`
	DoNotAdvertise              json.RawMessage `json:"do_not_advertise,omitempty"`
}

// HTTPLoadBalancerCreate is the request body for creating a new HTTP load
// balancer.
type HTTPLoadBalancerCreate struct {
	Metadata ObjectMeta           `json:"metadata"`
	Spec     HTTPLoadBalancerSpec `json:"spec"`
}

// HTTPLoadBalancerReplace is the request body for replacing an existing HTTP
// load balancer.
type HTTPLoadBalancerReplace struct {
	Metadata ObjectMeta           `json:"metadata"`
	Spec     HTTPLoadBalancerSpec `json:"spec"`
}

// HTTPLoadBalancer is the full HTTP load balancer object returned by the F5 XC
// API. RawSpec holds the unparsed "spec" JSON from the server response and is
// excluded from marshalling (json:"-").
type HTTPLoadBalancer struct {
	Metadata       ObjectMeta           `json:"metadata"`
	SystemMetadata SystemMeta           `json:"system_metadata,omitempty"`
	Spec           HTTPLoadBalancerSpec `json:"spec"`
	RawSpec        json.RawMessage      `json:"-"`
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateHTTPLoadBalancer creates a new HTTP load balancer in the given namespace.
func (c *Client) CreateHTTPLoadBalancer(ctx context.Context, ns string, lb *HTTPLoadBalancerCreate) (*HTTPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	var result HTTPLoadBalancer
	if err := c.do(ctx, http.MethodPost, ResourceHTTPLoadBalancer, ns, "", lb, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHTTPLoadBalancer retrieves an HTTP load balancer by name from the given
// namespace. The returned HTTPLoadBalancer has RawSpec populated with the raw
// "spec" JSON.
func (c *Client) GetHTTPLoadBalancer(ctx context.Context, ns, name string) (*HTTPLoadBalancer, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceHTTPLoadBalancer, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result HTTPLoadBalancer
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceHTTPLoadBalancer replaces an existing HTTP load balancer identified
// by name.
func (c *Client) ReplaceHTTPLoadBalancer(ctx context.Context, ns, name string, lb *HTTPLoadBalancerReplace) (*HTTPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	lb.Metadata.Name = name
	var result HTTPLoadBalancer
	if err := c.do(ctx, http.MethodPut, ResourceHTTPLoadBalancer, ns, name, lb, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteHTTPLoadBalancer removes the HTTP load balancer identified by name
// from the given namespace.
func (c *Client) DeleteHTTPLoadBalancer(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceHTTPLoadBalancer, ns, name, nil, nil)
}

// ListHTTPLoadBalancers returns all HTTP load balancers in the given namespace.
func (c *Client) ListHTTPLoadBalancers(ctx context.Context, ns string) ([]*HTTPLoadBalancer, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceHTTPLoadBalancer, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[HTTPLoadBalancer](raw)
}
