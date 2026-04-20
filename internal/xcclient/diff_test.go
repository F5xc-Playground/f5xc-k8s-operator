package xcclient_test

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// xcObject is a helper to build a realistic XC API JSON payload.
// Fields left as nil are omitted.
type xcObject struct {
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	SystemMetadata  map[string]interface{} `json:"system_metadata,omitempty"`
	Spec            map[string]interface{} `json:"spec,omitempty"`
	Status          map[string]interface{} `json:"status,omitempty"`
	ReferringObjects []interface{}         `json:"referring_objects,omitempty"`
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return json.RawMessage(b)
}

// baseObject returns a realistic XC "get" response with server-managed fields.
func baseObject(t *testing.T) json.RawMessage {
	t.Helper()
	return mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":             "my-origin-pool",
			"namespace":        "default",
			"uid":              "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"resource_version": "1234",
			"labels": map[string]string{
				"env": "prod",
			},
		},
		SystemMetadata: map[string]interface{}{
			"uid":                    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"creation_timestamp":     "2024-01-01T00:00:00Z",
			"modification_timestamp": "2024-06-01T12:00:00Z",
			"tenant":                 "acmecorp",
		},
		Spec: map[string]interface{}{
			"origin_servers": []interface{}{
				map[string]interface{}{
					"public_ip": map[string]interface{}{
						"ip": "1.2.3.4",
					},
				},
			},
			"port": 443,
		},
		Status: map[string]interface{}{
			"condition": []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
		ReferringObjects: []interface{}{
			map[string]interface{}{
				"name":      "my-lb",
				"namespace": "default",
			},
		},
	})
}

// desiredObject returns a "desired state" payload as a controller would build it
// (no server-managed fields).
func desiredObject(t *testing.T) json.RawMessage {
	t.Helper()
	return mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
			"labels": map[string]string{
				"env": "prod",
			},
		},
		Spec: map[string]interface{}{
			"origin_servers": []interface{}{
				map[string]interface{}{
					"public_ip": map[string]interface{}{
						"ip": "1.2.3.4",
					},
				},
			},
			"port": 443,
		},
	})
}

// TestNeedsUpdate_Identical verifies that two objects that are logically equal
// (after stripping server-managed fields) return false.
func TestNeedsUpdate_Identical(t *testing.T) {
	current := baseObject(t)
	desired := desiredObject(t)

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, needs, "identical objects (after stripping) should not need update")
}

// TestNeedsUpdate_DifferentSpec verifies that a changed spec field returns true.
func TestNeedsUpdate_DifferentSpec(t *testing.T) {
	current := baseObject(t)

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		Spec: map[string]interface{}{
			"origin_servers": []interface{}{
				map[string]interface{}{
					"public_ip": map[string]interface{}{
						"ip": "9.9.9.9", // changed IP
					},
				},
			},
			"port": 443,
		},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.True(t, needs, "different spec should need update")
}

// TestNeedsUpdate_IgnoresSystemMetadata verifies that differing system_metadata
// values do not trigger an update.
func TestNeedsUpdate_IgnoresSystemMetadata(t *testing.T) {
	current := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		SystemMetadata: map[string]interface{}{
			"uid":                    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"creation_timestamp":     "2024-01-01T00:00:00Z",
			"modification_timestamp": "2024-06-01T12:00:00Z",
		},
		Spec: map[string]interface{}{"port": 80},
	})

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		// no system_metadata at all
		Spec: map[string]interface{}{"port": 80},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, needs, "system_metadata differences should be ignored")
}

// TestNeedsUpdate_IgnoresMetadataUID verifies that metadata.uid differences
// are stripped and do not trigger an update.
func TestNeedsUpdate_IgnoresMetadataUID(t *testing.T) {
	current := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":             "my-origin-pool",
			"namespace":        "default",
			"uid":              "server-assigned-uid",
			"resource_version": "42",
		},
		Spec: map[string]interface{}{"port": 80},
	})

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
			// no uid / resource_version
		},
		Spec: map[string]interface{}{"port": 80},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, needs, "metadata.uid and resource_version differences should be ignored")
}

// TestNeedsUpdate_DetectsLabelChange verifies that a changed label triggers an update.
func TestNeedsUpdate_DetectsLabelChange(t *testing.T) {
	current := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
			"labels": map[string]string{
				"env": "prod",
			},
		},
		Spec: map[string]interface{}{"port": 80},
	})

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
			"labels": map[string]string{
				"env": "staging", // changed label
			},
		},
		Spec: map[string]interface{}{"port": 80},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.True(t, needs, "changed labels should need update")
}

// TestNeedsUpdate_IgnoresStatus verifies that a status field present in current
// but absent in desired does not trigger an update.
func TestNeedsUpdate_IgnoresStatus(t *testing.T) {
	current := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		Spec: map[string]interface{}{"port": 80},
		Status: map[string]interface{}{
			"condition": "Ready",
		},
	})

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		Spec: map[string]interface{}{"port": 80},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, needs, "status field differences should be ignored")
}

// TestNeedsUpdate_IgnoresReferringObjects verifies that referring_objects present
// in current but absent in desired does not trigger an update.
func TestNeedsUpdate_IgnoresReferringObjects(t *testing.T) {
	current := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		Spec: map[string]interface{}{"port": 80},
		ReferringObjects: []interface{}{
			map[string]interface{}{
				"name":      "some-lb",
				"namespace": "default",
			},
		},
	})

	desired := mustMarshal(t, xcObject{
		Metadata: map[string]interface{}{
			"name":      "my-origin-pool",
			"namespace": "default",
		},
		Spec: map[string]interface{}{"port": 80},
	})

	needs, err := xcclient.NeedsUpdate(current, desired)
	require.NoError(t, err)
	assert.False(t, needs, "referring_objects differences should be ignored")
}

// TestNeedsUpdate_InvalidJSON verifies that malformed input returns an error.
func TestNeedsUpdate_InvalidJSON(t *testing.T) {
	_, err := xcclient.NeedsUpdate(json.RawMessage(`{bad json`), json.RawMessage(`{}`))
	assert.Error(t, err, "malformed current JSON should return an error")

	_, err = xcclient.NeedsUpdate(json.RawMessage(`{}`), json.RawMessage(`{bad json`))
	assert.Error(t, err, "malformed desired JSON should return an error")
}
