package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type APIDefinitionSpec struct {
	SwaggerSpecs              json.RawMessage `json:"swagger_specs,omitempty"`
	APIInventoryInclusionList json.RawMessage `json:"api_inventory_inclusion_list,omitempty"`
	APIInventoryExclusionList json.RawMessage `json:"api_inventory_exclusion_list,omitempty"`
	NonAPIEndpoints           json.RawMessage `json:"non_api_endpoints,omitempty"`
	MixedSchemaOrigin         json.RawMessage `json:"mixed_schema_origin,omitempty"`
	StrictSchemaOrigin        json.RawMessage `json:"strict_schema_origin,omitempty"`
}

type APIDefinitionCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     APIDefinitionSpec `json:"spec"`
}

type APIDefinitionReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     APIDefinitionSpec `json:"spec"`
}

type APIDefinition struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           APIDefinitionSpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

func (c *Client) CreateAPIDefinition(ctx context.Context, ns string, ad *APIDefinitionCreate) (*APIDefinition, error) {
	ad.Metadata.Namespace = ns
	var result APIDefinition
	if err := c.do(ctx, http.MethodPost, ResourceAPIDefinition, ns, "", ad, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAPIDefinition(ctx context.Context, ns, name string) (*APIDefinition, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceAPIDefinition, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result APIDefinition
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceAPIDefinition(ctx context.Context, ns, name string, ad *APIDefinitionReplace) (*APIDefinition, error) {
	ad.Metadata.Namespace = ns
	ad.Metadata.Name = name
	var result APIDefinition
	if err := c.do(ctx, http.MethodPut, ResourceAPIDefinition, ns, name, ad, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAPIDefinition(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceAPIDefinition, ns, name, nil, nil)
}

func (c *Client) ListAPIDefinitions(ctx context.Context, ns string) ([]*APIDefinition, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceAPIDefinition, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[APIDefinition](raw)
}
