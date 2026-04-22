package xcclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleCertificateCreate() *CertificateCreate {
	return &CertificateCreate{
		Metadata: ObjectMeta{Name: "test-cert", Namespace: "shared"},
		Spec: CertificateSpec{
			CertificateURL: "string:///dGVzdA==",
			PrivateKey: CertificatePrivKey{
				ClearSecretInfo: &ClearSecretInfo{
					URL:      "string:///a2V5",
					Provider: "",
				},
			},
		},
	}
}

func TestCertificate_CreateAndGet(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	created, err := client.CreateCertificate(context.Background(), "shared", sampleCertificateCreate())
	require.NoError(t, err)
	assert.Equal(t, "test-cert", created.Metadata.Name)

	got, err := client.GetCertificate(context.Background(), "shared", "test-cert")
	require.NoError(t, err)
	assert.Equal(t, "test-cert", got.Metadata.Name)
	assert.NotNil(t, got.RawSpec)
}

func TestCertificate_Replace(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	_, err := client.CreateCertificate(context.Background(), "shared", sampleCertificateCreate())
	require.NoError(t, err)

	replace := &CertificateReplace{
		Metadata: ObjectMeta{Name: "test-cert", Namespace: "shared", ResourceVersion: "rv-1"},
		Spec: CertificateSpec{
			CertificateURL: "string:///bmV3Y2VydA==",
			PrivateKey: CertificatePrivKey{
				ClearSecretInfo: &ClearSecretInfo{
					URL:      "string:///bmV3a2V5",
					Provider: "",
				},
			},
			DisableOcspStapling: json.RawMessage(`{}`),
		},
	}
	replaced, err := client.ReplaceCertificate(context.Background(), "shared", "test-cert", replace)
	require.NoError(t, err)
	assert.Equal(t, "test-cert", replaced.Metadata.Name)
}

func TestCertificate_Delete(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	_, err := client.CreateCertificate(context.Background(), "shared", sampleCertificateCreate())
	require.NoError(t, err)

	err = client.DeleteCertificate(context.Background(), "shared", "test-cert")
	require.NoError(t, err)

	_, err = client.GetCertificate(context.Background(), "shared", "test-cert")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCertificate_List(t *testing.T) {
	fake := testutil.NewFakeXCServer()
	defer fake.Close()
	client := newTestClient(t, fake.URL())

	for _, name := range []string{"cert-1", "cert-2"} {
		c := sampleCertificateCreate()
		c.Metadata.Name = name
		_, err := client.CreateCertificate(context.Background(), "shared", c)
		require.NoError(t, err)
	}

	list, err := client.ListCertificates(context.Background(), "shared")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
