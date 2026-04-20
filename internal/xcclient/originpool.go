package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Origin server types (OneOf pattern — exactly one field is set per instance)
// ---------------------------------------------------------------------------

// PublicIP identifies an origin server by a public IP address.
type PublicIP struct {
	IP string `json:"ip"`
}

// PublicName identifies an origin server by a public DNS name.
type PublicName struct {
	DNSName string `json:"dns_name"`
}

// PrivateIP identifies an origin server by a private IP address on a site.
type PrivateIP struct {
	IP   string     `json:"ip"`
	Site *ObjectRef `json:"site,omitempty"`
}

// PrivateName identifies an origin server by a private DNS name on a site.
type PrivateName struct {
	DNSName string     `json:"dns_name"`
	Site    *ObjectRef `json:"site,omitempty"`
}

// K8SService identifies an origin server by a Kubernetes service.
type K8SService struct {
	ServiceName      string     `json:"service_name"`
	ServiceNamespace string     `json:"service_namespace,omitempty"`
	Site             *ObjectRef `json:"site,omitempty"`
}

// ConsulService identifies an origin server by a Consul service on a site.
type ConsulService struct {
	ServiceName string     `json:"service_name"`
	Site        *ObjectRef `json:"site,omitempty"`
}

// OriginServer describes a single upstream endpoint. Exactly one of the
// pointer fields should be set (OneOf pattern).
type OriginServer struct {
	PublicIP      *PublicIP      `json:"public_ip,omitempty"`
	PublicName    *PublicName    `json:"public_name,omitempty"`
	PrivateIP     *PrivateIP     `json:"private_ip,omitempty"`
	PrivateName   *PrivateName   `json:"private_name,omitempty"`
	K8SService    *K8SService    `json:"k8s_service,omitempty"`
	ConsulService *ConsulService `json:"consul_service,omitempty"`
}

// ---------------------------------------------------------------------------
// OriginPool types
// ---------------------------------------------------------------------------

// OriginPoolSpec is the resource-specific payload for an F5 XC origin pool.
type OriginPoolSpec struct {
	OriginServers         []OriginServer  `json:"origin_servers,omitempty"`
	Port                  int             `json:"port,omitempty"`
	UseTLS                json.RawMessage `json:"use_tls,omitempty"`
	NoTLS                 json.RawMessage `json:"no_tls,omitempty"`
	LoadBalancerAlgorithm string          `json:"loadbalancer_algorithm,omitempty"`
	HealthCheck           []ObjectRef     `json:"healthcheck,omitempty"`
}

// OriginPoolCreate is the request body for creating a new origin pool.
type OriginPoolCreate struct {
	Metadata ObjectMeta     `json:"metadata"`
	Spec     OriginPoolSpec `json:"spec"`
}

// OriginPoolReplace is the request body for replacing an existing origin pool.
type OriginPoolReplace struct {
	Metadata ObjectMeta     `json:"metadata"`
	Spec     OriginPoolSpec `json:"spec"`
}

// OriginPool is the full origin pool object returned by the F5 XC API.
// RawSpec holds the unparsed "spec" JSON from the server response and is
// excluded from marshalling (json:"-").
type OriginPool struct {
	Metadata       ObjectMeta     `json:"metadata"`
	SystemMetadata SystemMeta     `json:"system_metadata,omitempty"`
	Spec           OriginPoolSpec `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

// CreateOriginPool creates a new origin pool in the given namespace.
func (c *Client) CreateOriginPool(ctx context.Context, ns string, pool *OriginPoolCreate) (*OriginPool, error) {
	var result OriginPool
	if err := c.do(ctx, http.MethodPost, ResourceOriginPool, ns, "", pool, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOriginPool retrieves an origin pool by name from the given namespace.
// The returned OriginPool has RawSpec populated with the raw "spec" JSON.
func (c *Client) GetOriginPool(ctx context.Context, ns, name string) (*OriginPool, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceOriginPool, ns, name, nil, &raw); err != nil {
		return nil, err
	}

	var result OriginPool
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshalling origin pool: %w", err)
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceOriginPool replaces an existing origin pool identified by name.
func (c *Client) ReplaceOriginPool(ctx context.Context, ns, name string, pool *OriginPoolReplace) (*OriginPool, error) {
	var result OriginPool
	if err := c.do(ctx, http.MethodPut, ResourceOriginPool, ns, name, pool, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteOriginPool removes the origin pool identified by name from the given
// namespace.
func (c *Client) DeleteOriginPool(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceOriginPool, ns, name, nil, nil)
}

// ListOriginPools returns all origin pools in the given namespace.
func (c *Client) ListOriginPools(ctx context.Context, ns string) ([]*OriginPool, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceOriginPool, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[OriginPool](raw)
}
