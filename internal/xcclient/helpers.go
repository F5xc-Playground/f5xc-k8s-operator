package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// extractRawSpec pulls the "spec" field out of the raw JSON envelope returned
// by the server. Returns nil if the envelope cannot be decoded or the spec
// field is absent.
func extractRawSpec(raw json.RawMessage) json.RawMessage {
	var env struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	return env.Spec
}

// unmarshalList decodes a standard F5 XC list response ({"items":[...]}) and
// unmarshals each item into *T, returning a slice of pointers.
func unmarshalList[T any](data []byte) ([]*T, error) {
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshalling list response: %w", err)
	}
	out := make([]*T, 0, len(resp.Items))
	for i, raw := range resp.Items {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("unmarshalling list item %d: %w", i, err)
		}
		out = append(out, &item)
	}
	return out, nil
}

// crudList issues a GET to the list endpoint and decodes the items into []*T.
func crudList[T any](ctx context.Context, c *Client, resource, namespace string) ([]*T, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, resource, namespace, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[T](raw)
}
