package xcclient_test

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObjectMetaJSONRoundTrip verifies that ObjectMeta marshals and unmarshals
// correctly, preserving name, namespace, and labels.
func TestObjectMetaJSONRoundTrip(t *testing.T) {
	original := xcclient.ObjectMeta{
		Name:      "my-pool",
		Namespace: "default",
		Labels:    map[string]string{"env": "prod", "team": "platform"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded xcclient.ObjectMeta
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Namespace, decoded.Namespace)
	assert.Equal(t, original.Labels, decoded.Labels)
}

// TestObjectMetaResourceVersionOmitted verifies that resource_version is omitted
// from JSON when empty, and included when set.
func TestObjectMetaResourceVersionOmitted(t *testing.T) {
	t.Run("omitted when empty", func(t *testing.T) {
		meta := xcclient.ObjectMeta{Name: "test-obj"}
		data, err := json.Marshal(meta)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &raw))
		_, present := raw["resource_version"]
		assert.False(t, present, "resource_version should be absent when empty")
	})

	t.Run("included when set", func(t *testing.T) {
		meta := xcclient.ObjectMeta{Name: "test-obj", ResourceVersion: "42"}
		data, err := json.Marshal(meta)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &raw))
		_, present := raw["resource_version"]
		assert.True(t, present, "resource_version should be present when set")

		var version string
		require.NoError(t, json.Unmarshal(raw["resource_version"], &version))
		assert.Equal(t, "42", version)
	})
}

// TestSystemMetaDeserialization verifies that SystemMeta deserializes correctly
// from a JSON payload.
func TestSystemMetaDeserialization(t *testing.T) {
	payload := `{
		"uid": "abc-123",
		"creation_timestamp": "2024-01-15T10:00:00Z",
		"modification_timestamp": "2024-06-01T12:30:00Z",
		"creator_id": "user@example.com",
		"creator_class": "USER",
		"tenant": "acme-corp"
	}`

	var sm xcclient.SystemMeta
	require.NoError(t, json.Unmarshal([]byte(payload), &sm))

	assert.Equal(t, "abc-123", sm.UID)
	assert.Equal(t, "2024-01-15T10:00:00Z", sm.CreationTimestamp)
	assert.Equal(t, "2024-06-01T12:30:00Z", sm.ModificationTimestamp)
	assert.Equal(t, "user@example.com", sm.CreatorID)
	assert.Equal(t, "USER", sm.CreatorClass)
	assert.Equal(t, "acme-corp", sm.Tenant)
}

// TestObjectRefRoundTrip verifies that ObjectRef marshals and unmarshals correctly.
func TestObjectRefRoundTrip(t *testing.T) {
	original := xcclient.ObjectRef{
		Name:      "my-firewall",
		Namespace: "app-ns",
		Tenant:    "my-tenant",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded xcclient.ObjectRef
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original, decoded)
}

// TestObjectRefOmitsEmptyFields verifies that optional ObjectRef fields are
// omitted from JSON when empty.
func TestObjectRefOmitsEmptyFields(t *testing.T) {
	ref := xcclient.ObjectRef{Name: "minimal-ref"}
	data, err := json.Marshal(ref)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))

	_, hasNamespace := raw["namespace"]
	_, hasTenant := raw["tenant"]
	assert.False(t, hasNamespace, "namespace should be omitted when empty")
	assert.False(t, hasTenant, "tenant should be omitted when empty")
}

// TestResourcePathConstants verifies that all resource path constants have the
// exact expected values.
func TestResourcePathConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"OriginPool", xcclient.ResourceOriginPool, "origin_pools"},
		{"HTTPLoadBalancer", xcclient.ResourceHTTPLoadBalancer, "http_loadbalancers"},
		{"TCPLoadBalancer", xcclient.ResourceTCPLoadBalancer, "tcp_loadbalancers"},
		{"AppFirewall", xcclient.ResourceAppFirewall, "app_firewalls"},
		{"HealthCheck", xcclient.ResourceHealthCheck, "healthchecks"},
		{"ServicePolicy", xcclient.ResourceServicePolicy, "service_policys"},
		{"RateLimiter", xcclient.ResourceRateLimiter, "rate_limiters"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.constant)
		})
	}
}
