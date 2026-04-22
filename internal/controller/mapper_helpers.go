package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
)

// marshalJSON marshals v to JSON. It panics on error because callers only pass
// types that are known to be serialisable.
func marshalJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// emptyObjectJSON is a pre-built JSON representation of an empty object — used
// as a sentinel value in F5XC "oneof" fields.
var emptyObjectJSON = json.RawMessage(`{}`)

// ---------------------------------------------------------------------------
// Wire-format types (snake_case JSON tags — these mirror the F5XC REST API)
// ---------------------------------------------------------------------------

type xcCustomTLSSecurity struct {
	CipherSuites []string `json:"cipher_suites,omitempty"`
}

type xcUseMTLS struct {
	TrustedCAURL string `json:"trusted_ca_url,omitempty"`
}

type xcTLSCertificateRef struct {
	CertificateURL string             `json:"certificate_url,omitempty"`
	PrivateKey     xcCertificatePrivKey `json:"private_key,omitempty"`
}

type xcCertificatePrivKey struct {
	ClearSecretInfo *xcClearSecretInfo `json:"clear_secret_info,omitempty"`
}

type xcClearSecretInfo struct {
	URL      string `json:"url,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type xcAdvertiseOnPublic struct {
	PublicIP *xcObjectRef `json:"public_ip,omitempty"`
}

type xcAdvertiseCustom struct {
	AdvertiseWhere []xcAdvertiseWhere `json:"advertise_where,omitempty"`
}

type xcAdvertiseWhere struct {
	Site           *xcAdvertiseSite          `json:"site,omitempty"`
	VirtualSite    *xcAdvertiseSite          `json:"virtual_site,omitempty"`
	VirtualNetwork *xcAdvertiseVirtualNetwork `json:"virtual_network,omitempty"`
	Port           uint32                    `json:"port,omitempty"`
	UseDefaultPort *struct{}                 `json:"use_default_port,omitempty"`
}

type xcAdvertiseSite struct {
	Network   string      `json:"network,omitempty"`
	Site      xcObjectRef `json:"site,omitempty"`
	IPAddress string      `json:"ip_address,omitempty"`
}

type xcAdvertiseVirtualNetwork struct {
	DefaultVIP  *struct{} `json:"default_vip,omitempty"`
	SpecificVIP string    `json:"specific_vip,omitempty"`
}

type xcObjectRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

type xcPathMatcher struct {
	Prefix string `json:"prefix,omitempty"`
	Path   string `json:"path,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

type xcHeaderMatcher struct {
	Name        string `json:"name,omitempty"`
	Exact       string `json:"exact,omitempty"`
	Regex       string `json:"regex,omitempty"`
	Presence    bool   `json:"presence,omitempty"`
	InvertMatch bool   `json:"invert_match,omitempty"`
}

type xcHTTPMethodMatcher struct {
	Methods       []string `json:"methods,omitempty"`
	InvertMatcher bool     `json:"invert_matcher,omitempty"`
}

type xcIPMatcher struct {
	Prefixes    []string `json:"prefixes,omitempty"`
	InvertMatch bool     `json:"invert_match,omitempty"`
}

type xcASNMatcher struct {
	ASNumbers []uint32 `json:"as_numbers,omitempty"`
}

type xcLabelSelector struct {
	Expressions []string `json:"expressions,omitempty"`
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func mapXCObjectRef(ref *v1alpha1.ObjectRef) *xcObjectRef {
	if ref == nil {
		return nil
	}
	out := mapXCObjectRefVal(*ref)
	return &out
}

func mapXCObjectRefVal(ref v1alpha1.ObjectRef) xcObjectRef {
	return xcObjectRef{
		Name:      ref.Name,
		Namespace: ref.Namespace,
		Tenant:    ref.Tenant,
	}
}

func mapXCObjectRefs(refs []v1alpha1.ObjectRef) []xcObjectRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]xcObjectRef, len(refs))
	for i, r := range refs {
		out[i] = mapXCObjectRefVal(r)
	}
	return out
}

func mapXCPathMatcher(pm v1alpha1.PathMatcher) xcPathMatcher {
	return xcPathMatcher{
		Prefix: pm.Prefix,
		Path:   pm.Path,
		Regex:  pm.Regex,
	}
}

func mapXCHeaderMatchers(hms []v1alpha1.HeaderMatcher) []xcHeaderMatcher {
	if len(hms) == 0 {
		return nil
	}
	out := make([]xcHeaderMatcher, len(hms))
	for i, hm := range hms {
		out[i] = xcHeaderMatcher{
			Name:        hm.Name,
			Exact:       hm.Exact,
			Regex:       hm.Regex,
			Presence:    hm.Presence,
			InvertMatch: hm.InvertMatch,
		}
	}
	return out
}

func mapXCAdvertiseOnPublic(a *v1alpha1.AdvertiseOnPublic) json.RawMessage {
	if a == nil {
		return nil
	}
	return marshalJSON(xcAdvertiseOnPublic{
		PublicIP: mapXCObjectRef(a.PublicIP),
	})
}

func mapXCAdvertiseCustom(a *v1alpha1.AdvertiseCustom) json.RawMessage {
	if a == nil {
		return nil
	}
	wheres := make([]xcAdvertiseWhere, len(a.AdvertiseWhere))
	for i, w := range a.AdvertiseWhere {
		xw := xcAdvertiseWhere{
			Port: w.Port,
		}
		if w.UseDefaultPort != nil {
			xw.UseDefaultPort = &struct{}{}
		}
		if w.Site != nil {
			xw.Site = &xcAdvertiseSite{
				Network:   w.Site.Network,
				Site:      mapXCObjectRefVal(w.Site.Site),
				IPAddress: w.Site.IPAddress,
			}
		}
		if w.VirtualSite != nil {
			xw.VirtualSite = &xcAdvertiseSite{
				Network:   w.VirtualSite.Network,
				Site:      mapXCObjectRefVal(w.VirtualSite.Site),
				IPAddress: w.VirtualSite.IPAddress,
			}
		}
		if w.VirtualNetwork != nil {
			xvn := &xcAdvertiseVirtualNetwork{
				SpecificVIP: w.VirtualNetwork.SpecificVIP,
			}
			if w.VirtualNetwork.DefaultVIP != nil {
				xvn.DefaultVIP = &struct{}{}
			}
			xw.VirtualNetwork = xvn
		}
		wheres[i] = xw
	}
	return marshalJSON(xcAdvertiseCustom{AdvertiseWhere: wheres})
}

func mapXCCustomTLSSecurity(c *v1alpha1.CustomTLSSecurity) *xcCustomTLSSecurity {
	if c == nil {
		return nil
	}
	return &xcCustomTLSSecurity{CipherSuites: c.CipherSuites}
}

func mapXCUseMTLS(m *v1alpha1.UseMTLS) *xcUseMTLS {
	if m == nil {
		return nil
	}
	return &xcUseMTLS{TrustedCAURL: m.TrustedCAURL}
}

func mapXCTLSCertificateRefs(refs []v1alpha1.TLSCertificateRef) []xcTLSCertificateRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]xcTLSCertificateRef, len(refs))
	for i, r := range refs {
		xr := xcTLSCertificateRef{
			CertificateURL: r.CertificateURL,
		}
		if r.PrivateKey.ClearSecretInfo != nil {
			xr.PrivateKey = xcCertificatePrivKey{
				ClearSecretInfo: &xcClearSecretInfo{
					URL:      r.PrivateKey.ClearSecretInfo.URL,
					Provider: r.PrivateKey.ClearSecretInfo.Provider,
				},
			}
		}
		out[i] = xr
	}
	return out
}
