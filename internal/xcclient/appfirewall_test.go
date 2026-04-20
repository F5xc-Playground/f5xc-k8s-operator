package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a *Client pointing at the given server URL. It is
// shared by all resource-level test files in the xcclient package.
func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	cfg := Config{TenantURL: url, APIToken: "test-token"}
	client, err := NewClient(cfg, logr.Discard(), nil)
	require.NoError(t, err)
	return client
}

func TestAppFirewall_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &AppFirewallCreate{
		Metadata: ObjectMeta{Name: "base-waf", Namespace: "shared"},
		Spec: AppFirewallSpec{
			DefaultDetectionSettings: json.RawMessage(`{}`),
			Monitoring:               json.RawMessage(`{}`),
			UseDefaultBlockingPage:   json.RawMessage(`{}`),
			AllowAllResponseCodes:    json.RawMessage(`{}`),
			DefaultAnonymization:     json.RawMessage(`{}`),
			DefaultBotSetting:        json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateAppFirewall(context.Background(), "shared", create)
	require.NoError(t, err)
	assert.Equal(t, "base-waf", created.Metadata.Name)

	got, err := client.GetAppFirewall(context.Background(), "shared", "base-waf")
	require.NoError(t, err)
	assert.Equal(t, "base-waf", got.Metadata.Name)
}

func TestAppFirewall_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	_, err := client.CreateAppFirewall(context.Background(), "shared", &AppFirewallCreate{
		Metadata: ObjectMeta{Name: "waf-del", Namespace: "shared"},
		Spec:     AppFirewallSpec{Monitoring: json.RawMessage(`{}`)},
	})
	require.NoError(t, err)

	err = client.DeleteAppFirewall(context.Background(), "shared", "waf-del")
	require.NoError(t, err)

	_, err = client.GetAppFirewall(context.Background(), "shared", "waf-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAppFirewall_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"waf-1", "waf-2"} {
		_, err := client.CreateAppFirewall(context.Background(), "shared", &AppFirewallCreate{
			Metadata: ObjectMeta{Name: name, Namespace: "shared"},
			Spec:     AppFirewallSpec{Monitoring: json.RawMessage(`{}`)},
		})
		require.NoError(t, err)
	}

	list, err := client.ListAppFirewalls(context.Background(), "shared")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
