package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type MaliciousUserMitigationSpec struct {
	MitigationType json.RawMessage `json:"mitigation_type,omitempty"`
}

type MaliciousUserMitigationCreate struct {
	Metadata ObjectMeta                  `json:"metadata"`
	Spec     MaliciousUserMitigationSpec `json:"spec"`
}

type MaliciousUserMitigationReplace struct {
	Metadata ObjectMeta                  `json:"metadata"`
	Spec     MaliciousUserMitigationSpec `json:"spec"`
}

type MaliciousUserMitigation struct {
	Metadata       ObjectMeta                  `json:"metadata"`
	SystemMetadata SystemMeta                  `json:"system_metadata,omitempty"`
	Spec           MaliciousUserMitigationSpec `json:"spec"`
	RawSpec        json.RawMessage             `json:"-"`
}

func (c *Client) CreateMaliciousUserMitigation(ctx context.Context, ns string, m *MaliciousUserMitigationCreate) (*MaliciousUserMitigation, error) {
	m.Metadata.Namespace = ns
	var result MaliciousUserMitigation
	if err := c.do(ctx, http.MethodPost, ResourceMaliciousUserMitigation, ns, "", m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetMaliciousUserMitigation(ctx context.Context, ns, name string) (*MaliciousUserMitigation, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceMaliciousUserMitigation, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result MaliciousUserMitigation
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceMaliciousUserMitigation(ctx context.Context, ns, name string, m *MaliciousUserMitigationReplace) (*MaliciousUserMitigation, error) {
	m.Metadata.Namespace = ns
	m.Metadata.Name = name
	var result MaliciousUserMitigation
	if err := c.do(ctx, http.MethodPut, ResourceMaliciousUserMitigation, ns, name, m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteMaliciousUserMitigation(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceMaliciousUserMitigation, ns, name, nil, nil)
}

func (c *Client) ListMaliciousUserMitigations(ctx context.Context, ns string) ([]*MaliciousUserMitigation, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceMaliciousUserMitigation, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[MaliciousUserMitigation](raw)
}
