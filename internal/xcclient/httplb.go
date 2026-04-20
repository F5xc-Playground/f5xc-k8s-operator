package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// HTTPLoadBalancerSpec is the resource-specific payload for an F5 XC HTTP
// load balancer. Each group of fields represents a OneOf in the F5 XC API:
// exactly one field in each group should be set per object.
type HTTPLoadBalancerSpec struct {
	Domains           []string        `json:"domains,omitempty"`
	DefaultRoutePools []RoutePool     `json:"default_route_pools,omitempty"`
	Routes            json.RawMessage `json:"routes,omitempty"`

	// TLS — OneOf: http, https, https_auto_cert
	HTTP          json.RawMessage `json:"http,omitempty"`
	HTTPS         json.RawMessage `json:"https,omitempty"`
	HTTPSAutoCert json.RawMessage `json:"https_auto_cert,omitempty"`

	// WAF — OneOf: disable_waf (empty-object sentinel) or app_firewall (ObjectRef)
	DisableWAF  json.RawMessage `json:"disable_waf,omitempty"`
	AppFirewall *ObjectRef      `json:"app_firewall,omitempty"`

	// Bot defense — OneOf: disable_bot_defense, bot_defense
	DisableBotDefense json.RawMessage `json:"disable_bot_defense,omitempty"`
	BotDefense        json.RawMessage `json:"bot_defense,omitempty"`

	// API discovery — OneOf: disable_api_discovery, enable_api_discovery
	DisableAPIDiscovery json.RawMessage `json:"disable_api_discovery,omitempty"`
	EnableAPIDiscovery  json.RawMessage `json:"enable_api_discovery,omitempty"`

	// IP reputation — OneOf: disable_ip_reputation, enable_ip_reputation
	DisableIPReputation json.RawMessage `json:"disable_ip_reputation,omitempty"`
	EnableIPReputation  json.RawMessage `json:"enable_ip_reputation,omitempty"`

	// Rate limit — OneOf: disable_rate_limit, rate_limit
	DisableRateLimit json.RawMessage `json:"disable_rate_limit,omitempty"`
	RateLimit        json.RawMessage `json:"rate_limit,omitempty"`

	// Challenge — OneOf: no_challenge, js_challenge, captcha_challenge, policy_based_challenge
	NoChallenge          json.RawMessage `json:"no_challenge,omitempty"`
	JSChallenge          json.RawMessage `json:"js_challenge,omitempty"`
	CaptchaChallenge     json.RawMessage `json:"captcha_challenge,omitempty"`
	PolicyBasedChallenge json.RawMessage `json:"policy_based_challenge,omitempty"`

	// LB algorithm — OneOf: round_robin, least_active, random, source_ip_stickiness,
	// cookie_stickiness, ring_hash
	RoundRobin         json.RawMessage `json:"round_robin,omitempty"`
	LeastActive        json.RawMessage `json:"least_active,omitempty"`
	Random             json.RawMessage `json:"random,omitempty"`
	SourceIPStickiness json.RawMessage `json:"source_ip_stickiness,omitempty"`
	CookieStickiness   json.RawMessage `json:"cookie_stickiness,omitempty"`
	RingHash           json.RawMessage `json:"ring_hash,omitempty"`

	// Advertise — OneOf: advertise_on_public_default_vip, advertise_on_public,
	// advertise_custom, do_not_advertise
	AdvertiseOnPublicDefaultVIP json.RawMessage `json:"advertise_on_public_default_vip,omitempty"`
	AdvertiseOnPublic           json.RawMessage `json:"advertise_on_public,omitempty"`
	AdvertiseCustom             json.RawMessage `json:"advertise_custom,omitempty"`
	DoNotAdvertise              json.RawMessage `json:"do_not_advertise,omitempty"`

	// Service policies — OneOf: service_policies_from_namespace, active_service_policies,
	// no_service_policies
	ServicePoliciesFromNamespace json.RawMessage `json:"service_policies_from_namespace,omitempty"`
	ActiveServicePolicies        json.RawMessage `json:"active_service_policies,omitempty"`
	NoServicePolicies            json.RawMessage `json:"no_service_policies,omitempty"`

	// User ID — OneOf: user_id_client_ip
	UserIDClientIP json.RawMessage `json:"user_id_client_ip,omitempty"`
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
