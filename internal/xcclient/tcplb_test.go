package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
)

func TestTCPLoadBalancer_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	create := &TCPLoadBalancerCreate{
		Metadata: ObjectMeta{Name: "tcp-lb", Namespace: "default"},
		Spec: TCPLoadBalancerSpec{
			Domains:    []string{"tcp.example.com"},
			ListenPort: 5432,
			OriginPoolWeights: []RoutePool{
				{Pool: ObjectRef{Name: "db-pool", Namespace: "default"}, Weight: 1},
			},
			AdvertiseOnPublicDefaultVIP: json.RawMessage(`{}`),
		},
	}

	created, err := client.CreateTCPLoadBalancer(context.Background(), "default", create)
	require.NoError(t, err)
	assert.Equal(t, "tcp-lb", created.Metadata.Name)

	got, err := client.GetTCPLoadBalancer(context.Background(), "default", "tcp-lb")
	require.NoError(t, err)
	assert.Equal(t, "tcp-lb", got.Metadata.Name)
}

func TestTCPLoadBalancer_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	client.CreateTCPLoadBalancer(context.Background(), "default", &TCPLoadBalancerCreate{ //nolint:errcheck
		Metadata: ObjectMeta{Name: "tcp-del", Namespace: "default"},
		Spec:     TCPLoadBalancerSpec{ListenPort: 443},
	})

	err := client.DeleteTCPLoadBalancer(context.Background(), "default", "tcp-del")
	require.NoError(t, err)

	_, err = client.GetTCPLoadBalancer(context.Background(), "default", "tcp-del")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTCPLoadBalancer_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"tcp-1", "tcp-2"} {
		client.CreateTCPLoadBalancer(context.Background(), "default", &TCPLoadBalancerCreate{ //nolint:errcheck
			Metadata: ObjectMeta{Name: name, Namespace: "default"},
			Spec:     TCPLoadBalancerSpec{ListenPort: 443},
		})
	}

	list, err := client.ListTCPLoadBalancers(context.Background(), "default")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
