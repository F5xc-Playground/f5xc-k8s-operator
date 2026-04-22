package v1alpha1

type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

type RoutePool struct {
	Pool     ObjectRef `json:"pool"`
	Weight   *uint32   `json:"weight,omitempty"`
	Priority *uint32   `json:"priority,omitempty"`
}

type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// CustomTLSSecurity holds a custom TLS cipher suite list.
type CustomTLSSecurity struct {
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

// UseMTLS enables mutual TLS with the given trusted CA URL.
type UseMTLS struct {
	TrustedCAURL string `json:"trustedCAURL,omitempty"`
}

// TLSCertificateRef references a TLS certificate and its private key.
type TLSCertificateRef struct {
	CertificateURL string             `json:"certificateURL,omitempty"`
	PrivateKey     CertificatePrivKey `json:"privateKey,omitempty"`
}

// CertificatePrivKey holds private-key material for a TLS certificate.
type CertificatePrivKey struct {
	ClearSecretInfo *ClearSecretInfo `json:"clearSecretInfo,omitempty"`
}

// ClearSecretInfo identifies a secret stored in clear text.
type ClearSecretInfo struct {
	URL      string `json:"url,omitempty"`
	Provider string `json:"provider,omitempty"`
}

// AdvertiseOnPublic advertises the load balancer on a specific public IP.
type AdvertiseOnPublic struct {
	PublicIP *ObjectRef `json:"publicIP,omitempty"`
}

// AdvertiseCustom advertises the load balancer on one or more custom locations.
type AdvertiseCustom struct {
	AdvertiseWhere []AdvertiseWhere `json:"advertiseWhere,omitempty"`
}

// EmptyObject is a named empty struct used as a sentinel/flag value in F5XC
// "oneof" fields where the API expects an empty JSON object ("{}").
type EmptyObject struct{}

// AdvertiseWhere describes a single advertise-where entry.
type AdvertiseWhere struct {
	Site           *AdvertiseSite           `json:"site,omitempty"`
	VirtualSite    *AdvertiseSite           `json:"virtualSite,omitempty"`
	VirtualNetwork *AdvertiseVirtualNetwork `json:"virtualNetwork,omitempty"`
	Port           uint32                   `json:"port,omitempty"`
	UseDefaultPort *EmptyObject             `json:"useDefaultPort,omitempty"`
}

// AdvertiseSite describes a site (or virtual-site) advertise location.
type AdvertiseSite struct {
	Network   string    `json:"network,omitempty"`
	Site      ObjectRef `json:"site,omitempty"`
	IPAddress string    `json:"ipAddress,omitempty"`
}

// AdvertiseVirtualNetwork describes a virtual-network advertise location.
type AdvertiseVirtualNetwork struct {
	DefaultVIP  *EmptyObject `json:"defaultVIP,omitempty"`
	SpecificVIP string       `json:"specificVIP,omitempty"`
}

// PathMatcher matches an HTTP request path.
type PathMatcher struct {
	Prefix string `json:"prefix,omitempty"`
	Path   string `json:"path,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

// HeaderMatcher matches an HTTP request header.
type HeaderMatcher struct {
	Name        string `json:"name"`
	Exact       string `json:"exact,omitempty"`
	Regex       string `json:"regex,omitempty"`
	Presence    bool   `json:"presence,omitempty"`
	InvertMatch bool   `json:"invertMatch,omitempty"`
}

// HTTPMethodMatcher matches HTTP request methods.
type HTTPMethodMatcher struct {
	Methods       []string `json:"methods,omitempty"`
	InvertMatcher bool     `json:"invertMatcher,omitempty"`
}

// IPMatcher matches source IP prefixes.
type IPMatcher struct {
	Prefixes    []string `json:"prefixes,omitempty"`
	InvertMatch bool     `json:"invertMatch,omitempty"`
}

// ASNMatcher matches source autonomous system numbers.
type ASNMatcher struct {
	ASNumbers []uint32 `json:"asNumbers,omitempty"`
}

// LabelSelector selects resources by label expressions.
type LabelSelector struct {
	Expressions []string `json:"expressions"`
}

// APIOperation identifies an API endpoint by HTTP method and path.
type APIOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
