package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcUserIdentificationRule struct {
	None                   *v1alpha1.EmptyObject `json:"none,omitempty"`
	ClientIP               *v1alpha1.EmptyObject `json:"client_ip,omitempty"`
	ClientASN              *v1alpha1.EmptyObject `json:"client_asn,omitempty"`
	ClientCity             *v1alpha1.EmptyObject `json:"client_city,omitempty"`
	ClientCountry          *v1alpha1.EmptyObject `json:"client_country,omitempty"`
	ClientRegion           *v1alpha1.EmptyObject `json:"client_region,omitempty"`
	CookieName             string                `json:"cookie_name,omitempty"`
	HTTPHeaderName         string                `json:"http_header_name,omitempty"`
	IPAndHTTPHeaderName    string                `json:"ip_and_http_header_name,omitempty"`
	IPAndTLSFingerprint    *v1alpha1.EmptyObject `json:"ip_and_tls_fingerprint,omitempty"`
	IPAndJA4TLSFingerprint *v1alpha1.EmptyObject `json:"ip_and_ja4_tls_fingerprint,omitempty"`
	TLSFingerprint         *v1alpha1.EmptyObject `json:"tls_fingerprint,omitempty"`
	JA4TLSFingerprint      *v1alpha1.EmptyObject `json:"ja4_tls_fingerprint,omitempty"`
	JWTClaimName           string                `json:"jwt_claim_name,omitempty"`
	QueryParamKey          string                `json:"query_param_key,omitempty"`
}

func buildUserIdentificationCreate(cr *v1alpha1.UserIdentification, xcNamespace string) *xcclient.UserIdentificationCreate {
	return &xcclient.UserIdentificationCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapUserIdentificationSpec(&cr.Spec),
	}
}

func buildUserIdentificationReplace(cr *v1alpha1.UserIdentification, xcNamespace, resourceVersion string) *xcclient.UserIdentificationReplace {
	return &xcclient.UserIdentificationReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapUserIdentificationSpec(&cr.Spec),
	}
}

func buildUserIdentificationDesiredSpecJSON(cr *v1alpha1.UserIdentification, xcNamespace string) (json.RawMessage, error) {
	create := buildUserIdentificationCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapUserIdentificationSpec(spec *v1alpha1.UserIdentificationSpec) xcclient.UserIdentificationSpec {
	var out xcclient.UserIdentificationSpec
	if len(spec.Rules) > 0 {
		var rules []xcUserIdentificationRule
		for _, r := range spec.Rules {
			rules = append(rules, xcUserIdentificationRule{
				None:                   r.None,
				ClientIP:               r.ClientIP,
				ClientASN:              r.ClientASN,
				ClientCity:             r.ClientCity,
				ClientCountry:          r.ClientCountry,
				ClientRegion:           r.ClientRegion,
				CookieName:             r.CookieName,
				HTTPHeaderName:         r.HTTPHeaderName,
				IPAndHTTPHeaderName:    r.IPAndHTTPHeaderName,
				IPAndTLSFingerprint:    r.IPAndTLSFingerprint,
				IPAndJA4TLSFingerprint: r.IPAndJA4TLSFingerprint,
				TLSFingerprint:         r.TLSFingerprint,
				JA4TLSFingerprint:      r.JA4TLSFingerprint,
				JWTClaimName:           r.JWTClaimName,
				QueryParamKey:          r.QueryParamKey,
			})
		}
		out.Rules = marshalJSON(rules)
	}
	return out
}
