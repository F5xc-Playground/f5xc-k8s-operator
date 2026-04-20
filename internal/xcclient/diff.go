package xcclient

import (
	"encoding/json"
	"reflect"
)

// serverManagedTopLevel is the set of top-level keys that are set by the F5 XC
// server and must be stripped before comparing current vs. desired state.
var serverManagedTopLevel = map[string]struct{}{
	"system_metadata":   {},
	"status":            {},
	"referring_objects": {},
	"create_form":       {},
	"replace_form":      {},
}

// serverManagedMetaFields is the set of keys inside "metadata" that are
// assigned by the server and must be stripped before comparing.
var serverManagedMetaFields = map[string]struct{}{
	"uid":              {},
	"resource_version": {},
}

// NeedsUpdate reports whether the current server-side object differs from the
// desired state in any user-managed field. It strips server-managed top-level
// keys (system_metadata, status, etc.) and server-managed metadata fields
// (uid, resource_version) from both sides before comparing, so that
// server-assigned fields never cause a spurious update.
func NeedsUpdate(current, desired json.RawMessage) (bool, error) {
	currentMap, err := unmarshalRaw(current)
	if err != nil {
		return false, err
	}
	desiredMap, err := unmarshalRaw(desired)
	if err != nil {
		return false, err
	}

	stripServerManaged(currentMap)
	stripServerManaged(desiredMap)

	stripMetaFields(currentMap)
	stripMetaFields(desiredMap)

	currentNorm, err := normalise(currentMap)
	if err != nil {
		return false, err
	}
	desiredNorm, err := normalise(desiredMap)
	if err != nil {
		return false, err
	}

	return !reflect.DeepEqual(currentNorm, desiredNorm), nil
}

// stripServerManaged removes the well-known server-managed top-level keys from m.
func stripServerManaged(m map[string]json.RawMessage) {
	for k := range serverManagedTopLevel {
		delete(m, k)
	}
}

// stripMetaFields removes the well-known server-assigned fields from the nested
// "metadata" object within m, if present.
func stripMetaFields(m map[string]json.RawMessage) {
	raw, ok := m["metadata"]
	if !ok {
		return
	}

	meta, err := unmarshalRaw(raw)
	if err != nil {
		return
	}

	for k := range serverManagedMetaFields {
		delete(meta, k)
	}

	b, err := json.Marshal(meta)
	if err != nil {
		return
	}
	m["metadata"] = json.RawMessage(b)
}

// unmarshalRaw decodes a JSON object into a map whose values are kept as
// raw JSON so that subsequent per-key manipulation is cheap.
func unmarshalRaw(b json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// normalise re-marshals m to JSON and then unmarshals it into an interface{}
// so that reflect.DeepEqual operates on a fully decoded, type-normalised tree
// (numbers become float64, arrays become []interface{}, etc.).
func normalise(m map[string]json.RawMessage) (interface{}, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}
