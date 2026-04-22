package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildCertificateCreate(cr *v1alpha1.Certificate, xcNamespace string, certPEM, keyPEM []byte) *xcclient.CertificateCreate {
	return &xcclient.CertificateCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapCertificateSpec(&cr.Spec, certPEM, keyPEM),
	}
}

func buildCertificateReplace(cr *v1alpha1.Certificate, xcNamespace, resourceVersion string, certPEM, keyPEM []byte) *xcclient.CertificateReplace {
	return &xcclient.CertificateReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapCertificateSpec(&cr.Spec, certPEM, keyPEM),
	}
}

func buildCertificateDesiredSpecJSON(cr *v1alpha1.Certificate, xcNamespace string, certPEM, keyPEM []byte) (json.RawMessage, error) {
	create := buildCertificateCreate(cr, xcNamespace, certPEM, keyPEM)
	return json.Marshal(create.Spec)
}

func mapCertificateSpec(spec *v1alpha1.CertificateSpec, certPEM, keyPEM []byte) xcclient.CertificateSpec {
	out := xcclient.CertificateSpec{
		CertificateURL: fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(certPEM)),
		PrivateKey: xcclient.CertificatePrivKey{
			ClearSecretInfo: &xcclient.ClearSecretInfo{
				URL:      fmt.Sprintf("string:///%s", base64.StdEncoding.EncodeToString(keyPEM)),
				Provider: "",
			},
		},
	}

	if spec.CustomHashAlgorithms != nil {
		out.CustomHashAlgorithms = json.RawMessage(spec.CustomHashAlgorithms.Raw)
	}
	if spec.DisableOcspStapling != nil {
		out.DisableOcspStapling = json.RawMessage(spec.DisableOcspStapling.Raw)
	}
	if spec.UseSystemDefaults != nil {
		out.UseSystemDefaults = json.RawMessage(spec.UseSystemDefaults.Raw)
	}

	return out
}
