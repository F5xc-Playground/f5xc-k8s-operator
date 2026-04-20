package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

// AppFirewallSpec holds the configuration for an F5 XC App Firewall object.
// Each group of fields is a OneOf: at most one field in each group should be
// set. Fields are kept as json.RawMessage so callers can supply arbitrary
// nested objects without losing fidelity.
type AppFirewallSpec struct {
	// Detection — OneOf: DefaultDetectionSettings | DetectionSettings
	DefaultDetectionSettings json.RawMessage `json:"default_detection_settings,omitempty"`
	DetectionSettings        json.RawMessage `json:"detection_settings,omitempty"`

	// Enforcement mode — OneOf: Monitoring | Blocking
	Monitoring json.RawMessage `json:"monitoring,omitempty"`
	Blocking   json.RawMessage `json:"blocking,omitempty"`

	// Blocking page — OneOf: UseDefaultBlockingPage | BlockingPage
	UseDefaultBlockingPage json.RawMessage `json:"use_default_blocking_page,omitempty"`
	BlockingPage           json.RawMessage `json:"blocking_page,omitempty"`

	// Response codes — OneOf: AllowAllResponseCodes | AllowedResponseCodes
	AllowAllResponseCodes json.RawMessage `json:"allow_all_response_codes,omitempty"`
	AllowedResponseCodes  json.RawMessage `json:"allowed_response_codes,omitempty"`

	// Bot setting — OneOf: DefaultBotSetting | BotProtectionSetting
	DefaultBotSetting    json.RawMessage `json:"default_bot_setting,omitempty"`
	BotProtectionSetting json.RawMessage `json:"bot_protection_setting,omitempty"`

	// Anonymization — OneOf: DefaultAnonymization | CustomAnonymization
	DefaultAnonymization json.RawMessage `json:"default_anonymization,omitempty"`
	CustomAnonymization  json.RawMessage `json:"custom_anonymization,omitempty"`

	// Loadbalancer setting
	UseLoadbalancerSetting json.RawMessage `json:"use_loadbalancer_setting,omitempty"`
}

// AppFirewallCreate is the request body for creating an App Firewall.
type AppFirewallCreate struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     AppFirewallSpec `json:"spec"`
}

// AppFirewallReplace is the request body for replacing an App Firewall.
type AppFirewallReplace struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     AppFirewallSpec `json:"spec"`
}

// AppFirewall is the object returned by the F5 XC API for an App Firewall.
// RawSpec holds the verbatim spec JSON returned by the server (useful for
// round-tripping to NeedsUpdate / diff logic).
type AppFirewall struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           AppFirewallSpec `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

// CreateAppFirewall creates a new App Firewall in the given namespace.
func (c *Client) CreateAppFirewall(ctx context.Context, ns string, fw *AppFirewallCreate) (*AppFirewall, error) {
	fw.Metadata.Namespace = ns
	var result AppFirewall
	err := c.do(ctx, http.MethodPost, ResourceAppFirewall, ns, "", fw, &result)
	return &result, err
}

// GetAppFirewall fetches the App Firewall with the given name from the given
// namespace. RawSpec is populated from the verbatim server response.
func (c *Client) GetAppFirewall(ctx context.Context, ns, name string) (*AppFirewall, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceAppFirewall, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result AppFirewall
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

// ReplaceAppFirewall replaces an existing App Firewall.
func (c *Client) ReplaceAppFirewall(ctx context.Context, ns, name string, fw *AppFirewallReplace) (*AppFirewall, error) {
	fw.Metadata.Namespace = ns
	fw.Metadata.Name = name
	var result AppFirewall
	err := c.do(ctx, http.MethodPut, ResourceAppFirewall, ns, name, fw, &result)
	return &result, err
}

// DeleteAppFirewall deletes the App Firewall with the given name.
func (c *Client) DeleteAppFirewall(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceAppFirewall, ns, name, nil, nil)
}

// ListAppFirewalls returns all App Firewalls in the given namespace.
func (c *Client) ListAppFirewalls(ctx context.Context, ns string) ([]*AppFirewall, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceAppFirewall, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[AppFirewall](raw)
}
