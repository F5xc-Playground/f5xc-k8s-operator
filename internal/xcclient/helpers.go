package xcclient

import (
	"encoding/json"
	"fmt"
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
//
// The XC list endpoint returns items in a flat format with name/namespace at
// the top level and metadata: null. This function normalizes each item so
// that metadata.name and metadata.namespace are populated before unmarshaling
// into the target type.
func unmarshalList[T any](data []byte) ([]*T, error) {
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshalling list response: %w", err)
	}
	out := make([]*T, 0, len(resp.Items))
	for i, raw := range resp.Items {
		normalized := normalizeListItem(raw)
		var item T
		if err := json.Unmarshal(normalized, &item); err != nil {
			return nil, fmt.Errorf("unmarshalling list item %d: %w", i, err)
		}
		out = append(out, &item)
	}
	return out, nil
}

// normalizeListItem rewrites a flat-format list item (name/namespace at top
// level, metadata null) into the nested format our types expect.
func normalizeListItem(raw json.RawMessage) json.RawMessage {
	var probe struct {
		Name      string          `json:"name"`
		Namespace string          `json:"namespace"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil || probe.Name == "" {
		return raw
	}
	if probe.Metadata != nil && string(probe.Metadata) != "null" {
		return raw
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	meta, err := json.Marshal(ObjectMeta{Name: probe.Name, Namespace: probe.Namespace})
	if err != nil {
		return raw
	}
	m["metadata"] = meta
	rewritten, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return rewritten
}
