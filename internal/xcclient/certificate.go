package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type CertificateSpec struct {
	CertificateURL       string              `json:"certificate_url"`
	PrivateKey           CertificatePrivKey  `json:"private_key"`
	CustomHashAlgorithms json.RawMessage     `json:"custom_hash_algorithms,omitempty"`
	DisableOcspStapling  json.RawMessage     `json:"disable_ocsp_stapling,omitempty"`
	UseSystemDefaults    json.RawMessage     `json:"use_system_defaults,omitempty"`
}

type CertificatePrivKey struct {
	ClearSecretInfo *ClearSecretInfo `json:"clear_secret_info,omitempty"`
}

type ClearSecretInfo struct {
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

type CertificateCreate struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     CertificateSpec `json:"spec"`
}

type CertificateReplace struct {
	Metadata ObjectMeta      `json:"metadata"`
	Spec     CertificateSpec `json:"spec"`
}

type Certificate struct {
	Metadata       ObjectMeta      `json:"metadata"`
	SystemMetadata SystemMeta      `json:"system_metadata,omitempty"`
	Spec           CertificateSpec `json:"spec"`
	RawSpec        json.RawMessage `json:"-"`
}

func (c *Client) CreateCertificate(ctx context.Context, ns string, cert *CertificateCreate) (*Certificate, error) {
	cert.Metadata.Namespace = ns
	var result Certificate
	if err := c.do(ctx, http.MethodPost, ResourceCertificate, ns, "", cert, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetCertificate(ctx context.Context, ns, name string) (*Certificate, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceCertificate, ns, name, nil, &raw)
	if err != nil {
		return nil, err
	}
	var result Certificate
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceCertificate(ctx context.Context, ns, name string, cert *CertificateReplace) (*Certificate, error) {
	cert.Metadata.Namespace = ns
	cert.Metadata.Name = name
	var result Certificate
	if err := c.do(ctx, http.MethodPut, ResourceCertificate, ns, name, cert, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteCertificate(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceCertificate, ns, name, nil, nil)
}

func (c *Client) ListCertificates(ctx context.Context, ns string) ([]*Certificate, error) {
	var raw json.RawMessage
	err := c.do(ctx, http.MethodGet, ResourceCertificate, ns, "", nil, &raw)
	if err != nil {
		return nil, err
	}
	return unmarshalList[Certificate](raw)
}
