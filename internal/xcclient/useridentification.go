package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type UserIdentificationSpec struct {
	Rules json.RawMessage `json:"rules,omitempty"`
}

type UserIdentificationCreate struct {
	Metadata ObjectMeta             `json:"metadata"`
	Spec     UserIdentificationSpec `json:"spec"`
}

type UserIdentificationReplace struct {
	Metadata ObjectMeta             `json:"metadata"`
	Spec     UserIdentificationSpec `json:"spec"`
}

type UserIdentification struct {
	Metadata       ObjectMeta             `json:"metadata"`
	SystemMetadata SystemMeta             `json:"system_metadata,omitempty"`
	Spec           UserIdentificationSpec `json:"spec"`
	RawSpec        json.RawMessage        `json:"-"`
}

func (c *Client) CreateUserIdentification(ctx context.Context, ns string, ui *UserIdentificationCreate) (*UserIdentification, error) {
	ui.Metadata.Namespace = ns
	var result UserIdentification
	if err := c.do(ctx, http.MethodPost, ResourceUserIdentification, ns, "", ui, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetUserIdentification(ctx context.Context, ns, name string) (*UserIdentification, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceUserIdentification, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result UserIdentification
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceUserIdentification(ctx context.Context, ns, name string, ui *UserIdentificationReplace) (*UserIdentification, error) {
	ui.Metadata.Namespace = ns
	ui.Metadata.Name = name
	var result UserIdentification
	if err := c.do(ctx, http.MethodPut, ResourceUserIdentification, ns, name, ui, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteUserIdentification(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceUserIdentification, ns, name, nil, nil)
}

func (c *Client) ListUserIdentifications(ctx context.Context, ns string) ([]*UserIdentification, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceUserIdentification, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[UserIdentification](raw)
}
