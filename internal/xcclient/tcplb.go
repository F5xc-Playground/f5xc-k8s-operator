package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RoutePool associates an origin pool reference with a weight and priority.
// It is used by both HTTP and TCP load balancer specs.
type RoutePool struct {
	Pool     ObjectRef `json:"pool"`
	Weight   uint32    `json:"weight,omitempty"`
	Priority uint32    `json:"priority,omitempty"`
}

// TCPLoadBalancerSpec is the resource-specific payload for an F5 XC TCP load
// balancer.
type TCPLoadBalancerSpec struct {
	Domains           []string    `json:"domains,omitempty"`
	ListenPort        uint32      `json:"listen_port,omitempty"`
	OriginPoolWeights []RoutePool `json:"origin_pools_weights,omitempty"`

	// TLS — OneOf: tcp, tls_tcp, tls_tcp_auto_cert
	TCP            json.RawMessage `json:"tcp,omitempty"`
	TLSTCP         json.RawMessage `json:"tls_tcp,omitempty"`
	TLSTCPAutoCert json.RawMessage `json:"tls_tcp_auto_cert,omitempty"`

	// Advertise — OneOf: advertise_on_public_default_vip, advertise_on_public,
	// advertise_custom, do_not_advertise
	AdvertiseOnPublicDefaultVIP json.RawMessage `json:"advertise_on_public_default_vip,omitempty"`
	AdvertiseOnPublic           json.RawMessage `json:"advertise_on_public,omitempty"`
	AdvertiseCustom             json.RawMessage `json:"advertise_custom,omitempty"`
	DoNotAdvertise              json.RawMessage `json:"do_not_advertise,omitempty"`
}

// TCPLoadBalancerCreate is the request body for creating a new TCP load
// balancer.
type TCPLoadBalancerCreate struct {
	Metadata ObjectMeta          `json:"metadata"`
	Spec     TCPLoadBalancerSpec `json:"spec"`
}

// TCPLoadBalancerReplace is the request body for replacing an existing TCP
// load balancer.
type TCPLoadBalancerReplace struct {
	Metadata ObjectMeta          `json:"metadata"`
	Spec     TCPLoadBalancerSpec `json:"spec"`
}

// TCPLoadBalancer is the full TCP load balancer object returned by the F5 XC
// API. RawSpec holds the unparsed "spec" JSON from the server response and is
// excluded from marshalling (json:"-").
type TCPLoadBalancer struct {
	Metadata       ObjectMeta          `json:"metadata"`
	SystemMetadata SystemMeta          `json:"system_metadata,omitempty"`
	Spec           TCPLoadBalancerSpec `json:"spec"`
	RawSpec        json.RawMessage     `json:"-"`
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateTCPLoadBalancer creates a new TCP load balancer in the given namespace.
func (c *Client) CreateTCPLoadBalancer(ctx context.Context, ns string, lb *TCPLoadBalancerCreate) (*TCPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	var result TCPLoadBalancer
	if err := c.do(ctx, http.MethodPost, ResourceTCPLoadBalancer, ns, "", lb, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTCPLoadBalancer retrieves a TCP load balancer by name from the given
// namespace. The returned TCPLoadBalancer has RawSpec populated with the raw
// "spec" JSON.
func (c *Client) GetTCPLoadBalancer(ctx context.Context, ns, name string) (*TCPLoadBalancer, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceTCPLoadBalancer, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result TCPLoadBalancer
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshalling TCP load balancer: %w", err)
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceTCPLoadBalancer replaces an existing TCP load balancer identified by
// name.
func (c *Client) ReplaceTCPLoadBalancer(ctx context.Context, ns, name string, lb *TCPLoadBalancerReplace) (*TCPLoadBalancer, error) {
	lb.Metadata.Namespace = ns
	lb.Metadata.Name = name
	var result TCPLoadBalancer
	if err := c.do(ctx, http.MethodPut, ResourceTCPLoadBalancer, ns, name, lb, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTCPLoadBalancer removes the TCP load balancer identified by name from
// the given namespace.
func (c *Client) DeleteTCPLoadBalancer(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceTCPLoadBalancer, ns, name, nil, nil)
}

// ListTCPLoadBalancers returns all TCP load balancers in the given namespace.
func (c *Client) ListTCPLoadBalancers(ctx context.Context, ns string) ([]*TCPLoadBalancer, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceTCPLoadBalancer, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[TCPLoadBalancer](raw)
}
