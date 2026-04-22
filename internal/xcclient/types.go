package xcclient

import "encoding/json"

// Resource path constants — these are the exact API path plurals used in F5 XC
// REST endpoints. Some values are irregular plurals (e.g. service_policys).
const (
	ResourceOriginPool       = "origin_pools"
	ResourceHTTPLoadBalancer = "http_loadbalancers"
	ResourceTCPLoadBalancer  = "tcp_loadbalancers"
	ResourceAppFirewall      = "app_firewalls"
	ResourceHealthCheck      = "healthchecks"
	ResourceServicePolicy    = "service_policys" // Note: irregular plural
	ResourceRateLimiter      = "rate_limiters"
	ResourceCertificate      = "certificates"
)

// ObjectMeta holds the user-supplied metadata for an F5 XC object.
type ObjectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	Description     string            `json:"description,omitempty"`
	Disable         bool              `json:"disable,omitempty"`
	ResourceVersion string            `json:"resource_version,omitempty"`
	UID             string            `json:"uid,omitempty"`
}

// SystemMeta holds the server-assigned metadata returned by F5 XC for an object.
type SystemMeta struct {
	UID                   string `json:"uid,omitempty"`
	CreationTimestamp     string `json:"creation_timestamp,omitempty"`
	ModificationTimestamp string `json:"modification_timestamp,omitempty"`
	CreatorID             string `json:"creator_id,omitempty"`
	CreatorClass          string `json:"creator_class,omitempty"`
	Tenant                string `json:"tenant,omitempty"`
}

// ObjectRef is a lightweight reference to another F5 XC object.
type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

// ObjectEnvelope is the standard wrapper used when creating or retrieving a
// single F5 XC object. Spec holds the resource-specific payload verbatim.
type ObjectEnvelope struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           json.RawMessage `json:"spec"`
}

// ListResponse is the standard wrapper returned by F5 XC list endpoints.
type ListResponse struct {
	Items []json.RawMessage `json:"items"`
}
