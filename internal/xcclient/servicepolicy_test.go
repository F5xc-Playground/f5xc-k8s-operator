package xcclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleServicePolicyCreate(name, ns string) *ServicePolicyCreate {
	return &ServicePolicyCreate{
		Metadata: ObjectMeta{Name: name, Namespace: ns},
		Spec: ServicePolicySpec{
			AllowAllRequests: json.RawMessage(`{}`),
			AnyServer:        json.RawMessage(`{}`),
		},
	}
}

// TestServicePolicy_CreateAndGet verifies that a created ServicePolicy can be
// retrieved and that the returned metadata matches what was sent.
func TestServicePolicy_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	created, err := client.CreateServicePolicy(ctx, "default", sampleServicePolicyCreate("my-policy", "default"))
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "my-policy", created.Metadata.Name)
	assert.Equal(t, "default", created.Metadata.Namespace)

	got, err := client.GetServicePolicy(ctx, "default", "my-policy")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "my-policy", got.Metadata.Name)
	assert.Equal(t, "default", got.Metadata.Namespace)

	// Verify the irregular plural is used: the recorded path must contain
	// "service_policys" (not "service_policies").
	reqs := fake.Requests()
	require.NotEmpty(t, reqs)
	found := false
	for _, r := range reqs {
		if strings.Contains(r.Path, "service_policys") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one request path containing service_policys; got %v", reqs)
}

// TestServicePolicy_DeleteAndVerifyGone verifies that deleting a ServicePolicy
// causes a subsequent Get to return ErrNotFound.
func TestServicePolicy_DeleteAndVerifyGone(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	_, err := client.CreateServicePolicy(ctx, "default", sampleServicePolicyCreate("to-delete", "default"))
	require.NoError(t, err)

	err = client.DeleteServicePolicy(ctx, "default", "to-delete")
	require.NoError(t, err)

	_, err = client.GetServicePolicy(ctx, "default", "to-delete")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound, "expected ErrNotFound after delete, got %v", err)
}

// TestServicePolicy_ListCount verifies that listing returns all policies in a
// namespace.
func TestServicePolicy_ListCount(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())
	ctx := context.Background()

	for _, name := range []string{"sp-a", "sp-b", "sp-c"} {
		_, err := client.CreateServicePolicy(ctx, "prod", sampleServicePolicyCreate(name, "prod"))
		require.NoError(t, err)
	}

	policies, err := client.ListServicePolicies(ctx, "prod")
	require.NoError(t, err)
	assert.Len(t, policies, 3)
}
