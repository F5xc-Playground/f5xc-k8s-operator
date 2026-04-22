# Typed CRD Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all `*apiextensionsv1.JSON` (opaque JSON) fields across 6 CRDs with proper Go structs so users write native YAML instead of embedded JSON.

**Architecture:** CRD types get proper Go structs with camelCase JSON tags (the YAML schema). Mappers marshal these to snake_case `json.RawMessage` for xcclient (the API boundary). xcclient types stay as `json.RawMessage` — no changes. A shared `marshalJSON` helper and wire-format structs (snake_case tags) live in the mapper layer.

**Tech Stack:** Go, controller-gen (deepcopy + CRD generation), envtest (integration tests), F5 XC API (contract tests)

---

## File Structure

| File | Responsibility |
|------|---------------|
| `api/v1alpha1/shared_types.go` | Shared CRD types: advertise, TLS, matchers, label selectors (extend existing file) |
| `api/v1alpha1/certificate_types.go` | Replace 3 JSON fields with `*struct{}` / `*CustomHashAlgorithms` |
| `api/v1alpha1/originpool_types.go` | Replace 10 JSON fields with `*struct{}` / `*OriginPoolTLS` |
| `api/v1alpha1/appfirewall_types.go` | Replace 13 JSON fields with typed structs |
| `api/v1alpha1/servicepolicy_types.go` | Replace 8 JSON fields with typed structs |
| `api/v1alpha1/tcplb_types.go` | Replace 7 JSON fields with typed structs |
| `api/v1alpha1/httplb_types.go` | Replace 30+ JSON fields with typed structs |
| `internal/controller/mapper_helpers.go` | `marshalJSON` helper + shared wire-format types (snake_case) |
| `internal/controller/certificate_mapper.go` | Update mapper for typed Certificate fields |
| `internal/controller/originpool_mapper.go` | Update mapper for typed OriginPool fields |
| `internal/controller/appfirewall_mapper.go` | Update mapper for typed AppFirewall fields |
| `internal/controller/servicepolicy_mapper.go` | Update mapper for typed ServicePolicy fields |
| `internal/controller/tcplb_mapper.go` | Update mapper for typed TCPLoadBalancer fields |
| `internal/controller/httplb_mapper.go` | Update mapper for typed HTTPLoadBalancer fields |
| `internal/controller/*_mapper_test.go` | Update all 6 mapper test files |

No changes to `internal/xcclient/*.go` — the API boundary stays as `json.RawMessage`.

---

### Task 1: Shared Types and Mapper Helpers

**Files:**
- Modify: `api/v1alpha1/shared_types.go`
- Create: `internal/controller/mapper_helpers.go`

- [ ] **Step 1: Add shared CRD types to shared_types.go**

Add these types below the existing `ResourceRef` type in `api/v1alpha1/shared_types.go`:

```go
// --- TLS types (shared by OriginPool, TCPLoadBalancer, HTTPLoadBalancer) ---

type CustomTLSSecurity struct {
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

type UseMTLS struct {
	TrustedCAURL string `json:"trustedCAURL,omitempty"`
}

type TLSCertificateRef struct {
	CertificateURL string             `json:"certificateURL,omitempty"`
	PrivateKey     CertificatePrivKey `json:"privateKey,omitempty"`
}

type CertificatePrivKey struct {
	ClearSecretInfo *ClearSecretInfo `json:"clearSecretInfo,omitempty"`
}

type ClearSecretInfo struct {
	URL      string `json:"url"`
	Provider string `json:"provider,omitempty"`
}

// --- Advertise types (shared by TCPLoadBalancer, HTTPLoadBalancer) ---

type AdvertiseOnPublic struct {
	PublicIP *ObjectRef `json:"publicIP,omitempty"`
}

type AdvertiseCustom struct {
	AdvertiseWhere []AdvertiseWhere `json:"advertiseWhere"`
}

type AdvertiseWhere struct {
	Site           *AdvertiseSite           `json:"site,omitempty"`
	VirtualSite    *AdvertiseSite           `json:"virtualSite,omitempty"`
	VirtualNetwork *AdvertiseVirtualNetwork `json:"virtualNetwork,omitempty"`
	Port           uint32                   `json:"port,omitempty"`
	UseDefaultPort *struct{}                `json:"useDefaultPort,omitempty"`
}

type AdvertiseSite struct {
	Network   string    `json:"network,omitempty"`
	Site      ObjectRef `json:"site"`
	IPAddress string    `json:"ipAddress,omitempty"`
}

type AdvertiseVirtualNetwork struct {
	DefaultVIP  *struct{} `json:"defaultVIP,omitempty"`
	SpecificVIP string    `json:"specificVIP,omitempty"`
}

// --- Matcher types (shared by ServicePolicy, HTTPLoadBalancer) ---

type PathMatcher struct {
	Prefix string `json:"prefix,omitempty"`
	Path   string `json:"path,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

type HeaderMatcher struct {
	Name        string `json:"name"`
	Exact       string `json:"exact,omitempty"`
	Regex       string `json:"regex,omitempty"`
	Presence    bool   `json:"presence,omitempty"`
	InvertMatch bool   `json:"invertMatch,omitempty"`
}

type HTTPMethodMatcher struct {
	Methods       []string `json:"methods,omitempty"`
	InvertMatcher bool     `json:"invertMatcher,omitempty"`
}

type IPMatcher struct {
	Prefixes    []string `json:"prefixes,omitempty"`
	InvertMatch bool     `json:"invertMatch,omitempty"`
}

type ASNMatcher struct {
	ASNumbers []uint32 `json:"asNumbers,omitempty"`
}

type LabelSelector struct {
	Expressions []string `json:"expressions"`
}
```

- [ ] **Step 2: Create mapper_helpers.go with marshalJSON and shared wire-format types**

```go
package controller

import (
	"encoding/json"
	"fmt"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
)

func marshalJSON(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshalJSON: %v", err))
	}
	return raw
}

var emptyObjectJSON = json.RawMessage(`{}`)

// --- Shared wire-format types (snake_case JSON tags for XC API) ---

type xcCustomTLSSecurity struct {
	CipherSuites []string `json:"cipher_suites,omitempty"`
}

type xcUseMTLS struct {
	TrustedCAURL string `json:"trusted_ca_url,omitempty"`
}

type xcTLSCertificateRef struct {
	CertificateURL string              `json:"certificate_url,omitempty"`
	PrivateKey     xcCertificatePrivKey `json:"private_key,omitempty"`
}

type xcCertificatePrivKey struct {
	ClearSecretInfo *xcClearSecretInfo `json:"clear_secret_info,omitempty"`
}

type xcClearSecretInfo struct {
	URL      string `json:"url"`
	Provider string `json:"provider,omitempty"`
}

// --- Advertise wire types ---

type xcAdvertiseOnPublic struct {
	PublicIP *xcObjectRef `json:"public_ip,omitempty"`
}

type xcAdvertiseCustom struct {
	AdvertiseWhere []xcAdvertiseWhere `json:"advertise_where"`
}

type xcAdvertiseWhere struct {
	Site           *xcAdvertiseSite           `json:"site,omitempty"`
	VirtualSite    *xcAdvertiseSite           `json:"virtual_site,omitempty"`
	VirtualNetwork *xcAdvertiseVirtualNetwork `json:"virtual_network,omitempty"`
	Port           uint32                     `json:"port,omitempty"`
	UseDefaultPort *struct{}                  `json:"use_default_port,omitempty"`
}

type xcAdvertiseSite struct {
	Network   string      `json:"network,omitempty"`
	Site      xcObjectRef `json:"site"`
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

// --- Matcher wire types ---

type xcPathMatcher struct {
	Prefix string `json:"prefix,omitempty"`
	Path   string `json:"path,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

type xcHeaderMatcher struct {
	Name        string `json:"name"`
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
	Expressions []string `json:"expressions"`
}

// --- Conversion helpers ---

func mapXCObjectRef(ref *v1alpha1.ObjectRef) *xcObjectRef {
	if ref == nil {
		return nil
	}
	return &xcObjectRef{Name: ref.Name, Namespace: ref.Namespace, Tenant: ref.Tenant}
}

func mapXCObjectRefVal(ref v1alpha1.ObjectRef) xcObjectRef {
	return xcObjectRef{Name: ref.Name, Namespace: ref.Namespace, Tenant: ref.Tenant}
}

func mapXCObjectRefs(refs []v1alpha1.ObjectRef) []xcObjectRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]xcObjectRef, len(refs))
	for i, ref := range refs {
		out[i] = mapXCObjectRefVal(ref)
	}
	return out
}

func mapXCPathMatcher(pm v1alpha1.PathMatcher) xcPathMatcher {
	return xcPathMatcher{Prefix: pm.Prefix, Path: pm.Path, Regex: pm.Regex}
}

func mapXCHeaderMatchers(hms []v1alpha1.HeaderMatcher) []xcHeaderMatcher {
	if len(hms) == 0 {
		return nil
	}
	out := make([]xcHeaderMatcher, len(hms))
	for i, hm := range hms {
		out[i] = xcHeaderMatcher{
			Name: hm.Name, Exact: hm.Exact, Regex: hm.Regex,
			Presence: hm.Presence, InvertMatch: hm.InvertMatch,
		}
	}
	return out
}

func mapXCAdvertiseOnPublic(a *v1alpha1.AdvertiseOnPublic) json.RawMessage {
	return marshalJSON(xcAdvertiseOnPublic{
		PublicIP: mapXCObjectRef(a.PublicIP),
	})
}

func mapXCAdvertiseCustom(a *v1alpha1.AdvertiseCustom) json.RawMessage {
	wire := xcAdvertiseCustom{}
	for _, w := range a.AdvertiseWhere {
		xw := xcAdvertiseWhere{
			Port:           w.Port,
			UseDefaultPort: w.UseDefaultPort,
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
			xw.VirtualNetwork = &xcAdvertiseVirtualNetwork{
				DefaultVIP:  w.VirtualNetwork.DefaultVIP,
				SpecificVIP: w.VirtualNetwork.SpecificVIP,
			}
		}
		wire.AdvertiseWhere = append(wire.AdvertiseWhere, xw)
	}
	return marshalJSON(wire)
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
	for i, ref := range refs {
		out[i] = xcTLSCertificateRef{
			CertificateURL: ref.CertificateURL,
		}
		if ref.PrivateKey.ClearSecretInfo != nil {
			out[i].PrivateKey = xcCertificatePrivKey{
				ClearSecretInfo: &xcClearSecretInfo{
					URL:      ref.PrivateKey.ClearSecretInfo.URL,
					Provider: ref.PrivateKey.ClearSecretInfo.Provider,
				},
			}
		}
	}
	return out
}
```

- [ ] **Step 3: Run make generate to update deepcopy for new shared types**

Run: `make generate`
Expected: deepcopy methods regenerated for new types in shared_types.go

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/shared_types.go internal/controller/mapper_helpers.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add shared CRD types and mapper wire-format helpers"
```

---

### Task 2: Certificate — Type and Mapper Update (3 fields)

**Files:**
- Modify: `api/v1alpha1/certificate_types.go`
- Modify: `internal/controller/certificate_mapper.go`
- Modify: `internal/controller/certificate_mapper_test.go`

- [ ] **Step 1: Write the failing test — update certificate mapper test for typed fields**

In `internal/controller/certificate_mapper_test.go`, replace `TestBuildCertificateCreate_OCSPFields`:

```go
func TestBuildCertificateCreate_OCSPFields(t *testing.T) {
	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-ocsp", Namespace: "ns"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace:         "ns",
			SecretRef:           v1alpha1.SecretRef{Name: "tls"},
			DisableOcspStapling: &struct{}{},
		},
	}

	result := buildCertificateCreate(cr, "ns", testCertPEM, testKeyPEM)
	assert.JSONEq(t, `{}`, string(result.Spec.DisableOcspStapling))
	assert.Nil(t, result.Spec.CustomHashAlgorithms)
	assert.Nil(t, result.Spec.UseSystemDefaults)
}

func TestBuildCertificateCreate_CustomHashAlgorithms(t *testing.T) {
	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-hash", Namespace: "ns"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "ns",
			SecretRef:   v1alpha1.SecretRef{Name: "tls"},
			CustomHashAlgorithms: &v1alpha1.CustomHashAlgorithms{
				HashAlgorithms: []string{"SHA256", "SHA384"},
			},
		},
	}

	result := buildCertificateCreate(cr, "ns", testCertPEM, testKeyPEM)
	assert.JSONEq(t, `{"hash_algorithms":["SHA256","SHA384"]}`, string(result.Spec.CustomHashAlgorithms))
	assert.Nil(t, result.Spec.DisableOcspStapling)
	assert.Nil(t, result.Spec.UseSystemDefaults)
}
```

Remove the `apiextensionsv1` import from the test file since it's no longer needed.

- [ ] **Step 2: Run test to verify it fails (won't compile — types not yet updated)**

Run: `go test ./internal/controller/ -run TestBuildCertificateCreate_OCSP -v -count=1 2>&1 | head -20`
Expected: compilation error — `struct{}` not assignable to `*apiextensionsv1.JSON`

- [ ] **Step 3: Update certificate_types.go — replace JSON fields with typed structs**

Replace the OCSP fields in `CertificateSpec`:

```go
type CertificateSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// +kubebuilder:validation:Required
	SecretRef SecretRef `json:"secretRef"`

	// OCSP stapling choice — OneOf: CustomHashAlgorithms | DisableOcspStapling | UseSystemDefaults
	CustomHashAlgorithms *CustomHashAlgorithms `json:"customHashAlgorithms,omitempty"`
	DisableOcspStapling  *struct{}             `json:"disableOcspStapling,omitempty"`
	UseSystemDefaults    *struct{}             `json:"useSystemDefaults,omitempty"`
}

type CustomHashAlgorithms struct {
	HashAlgorithms []string `json:"hashAlgorithms"`
}
```

Remove the `apiextensionsv1` import from the file.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`
Expected: deepcopy updated for `CustomHashAlgorithms` and `CertificateSpec`

- [ ] **Step 5: Update certificate_mapper.go — marshal typed structs to snake_case JSON**

Replace the mapping logic in `mapCertificateSpec`:

```go
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
		out.CustomHashAlgorithms = marshalJSON(struct {
			HashAlgorithms []string `json:"hash_algorithms"`
		}{HashAlgorithms: spec.CustomHashAlgorithms.HashAlgorithms})
	}
	if spec.DisableOcspStapling != nil {
		out.DisableOcspStapling = emptyObjectJSON
	}
	if spec.UseSystemDefaults != nil {
		out.UseSystemDefaults = emptyObjectJSON
	}

	return out
}
```

Remove the unused `"encoding/json"` import if it becomes unused (the mapper no longer uses `json.RawMessage(spec.X.Raw)`).

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildCertificate -v -count=1`
Expected: all certificate mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/certificate_types.go internal/controller/certificate_mapper.go internal/controller/certificate_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(certificate): replace opaque JSON fields with typed structs"
```

---

### Task 3: OriginPool — Type and Mapper Update (10 fields)

**Files:**
- Modify: `api/v1alpha1/originpool_types.go`
- Modify: `internal/controller/originpool_mapper.go`
- Modify: `internal/controller/originpool_mapper_test.go`

- [ ] **Step 1: Write the failing test — update origin pool mapper test for typed fields**

In `internal/controller/originpool_mapper_test.go`, replace `TestBuildOriginPoolCreate_TLSPassthrough`:

```go
func TestBuildOriginPoolCreate_UseTLS(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			UseTLS: &v1alpha1.OriginPoolTLS{
				DefaultSecurity: &struct{}{},
			},
		},
	}

	result := buildOriginPoolCreate(cr, "ns", nil)
	assert.JSONEq(t, `{"tls_config":{"default_security":{}}}`, string(result.Spec.UseTLS))
}

func TestBuildOriginPoolCreate_NoTLS(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "notls-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 80,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			NoTLS: &struct{}{},
		},
	}

	result := buildOriginPoolCreate(cr, "ns", nil)
	assert.JSONEq(t, `{}`, string(result.Spec.NoTLS))
	assert.Nil(t, result.Spec.UseTLS)
}

func TestBuildOriginPoolCreate_InsideNetwork(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "net-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 80,
			OriginServers: []v1alpha1.OriginServer{
				{PrivateIP: &v1alpha1.PrivateIP{
					IP:            "10.0.0.1",
					Site:          &v1alpha1.ObjectRef{Name: "site1"},
					InsideNetwork: &struct{}{},
				}},
			},
		},
	}

	result := buildOriginPoolCreate(cr, "ns", nil)
	require.Len(t, result.Spec.OriginServers, 1)
	assert.JSONEq(t, `{}`, string(result.Spec.OriginServers[0].PrivateIP.InsideNetwork))
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildOriginPoolCreate_UseTLS -v -count=1 2>&1 | head -20`
Expected: compilation error

- [ ] **Step 3: Update originpool_types.go — replace JSON fields with typed structs**

Replace the JSON fields:

```go
type OriginPoolSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace           string         `json:"xcNamespace"`
	OriginServers         []OriginServer `json:"originServers"`
	Port                  int            `json:"port"`
	LoadBalancerAlgorithm string         `json:"loadBalancerAlgorithm,omitempty"`
	HealthChecks          []ObjectRef    `json:"healthChecks,omitempty"`

	// TLS OneOf: useTLS, noTLS
	UseTLS *OriginPoolTLS `json:"useTLS,omitempty"`
	NoTLS  *struct{}      `json:"noTLS,omitempty"`
}

type OriginPoolTLS struct {
	DefaultSecurity        *struct{}          `json:"defaultSecurity,omitempty"`
	LowSecurity            *struct{}          `json:"lowSecurity,omitempty"`
	MediumSecurity         *struct{}          `json:"mediumSecurity,omitempty"`
	CustomSecurity         *CustomTLSSecurity `json:"customSecurity,omitempty"`
	SNI                    string             `json:"sni,omitempty"`
	VolterraTrustedCA      *struct{}          `json:"volterraTrustedCA,omitempty"`
	TrustedCAURL           string             `json:"trustedCAURL,omitempty"`
	DisableSNI             *struct{}          `json:"disableSNI,omitempty"`
	UseServerVerification  *struct{}          `json:"useServerVerification,omitempty"`
	SkipServerVerification *struct{}          `json:"skipServerVerification,omitempty"`
	NoMTLS                 *struct{}          `json:"noMTLS,omitempty"`
}
```

Replace the network fields in `PrivateIP`, `PrivateName`, `K8SService`, `ConsulService`:

```go
type PrivateIP struct {
	IP             string     `json:"ip"`
	Site           *ObjectRef `json:"site,omitempty"`
	VirtualSite    *ObjectRef `json:"virtualSite,omitempty"`
	InsideNetwork  *struct{}  `json:"insideNetwork,omitempty"`
	OutsideNetwork *struct{}  `json:"outsideNetwork,omitempty"`
}

type PrivateName struct {
	DNSName        string     `json:"dnsName"`
	Site           *ObjectRef `json:"site,omitempty"`
	VirtualSite    *ObjectRef `json:"virtualSite,omitempty"`
	InsideNetwork  *struct{}  `json:"insideNetwork,omitempty"`
	OutsideNetwork *struct{}  `json:"outsideNetwork,omitempty"`
}

type K8SService struct {
	ServiceName      string     `json:"serviceName"`
	ServiceNamespace string     `json:"serviceNamespace,omitempty"`
	Site             *ObjectRef `json:"site,omitempty"`
	VirtualSite      *ObjectRef `json:"virtualSite,omitempty"`
	InsideNetwork    *struct{}  `json:"insideNetwork,omitempty"`
	OutsideNetwork   *struct{}  `json:"outsideNetwork,omitempty"`
}

type ConsulService struct {
	ServiceName    string     `json:"serviceName"`
	Site           *ObjectRef `json:"site,omitempty"`
	VirtualSite    *ObjectRef `json:"virtualSite,omitempty"`
	InsideNetwork  *struct{}  `json:"insideNetwork,omitempty"`
	OutsideNetwork *struct{}  `json:"outsideNetwork,omitempty"`
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`
Expected: deepcopy updated for `OriginPoolTLS` and updated origin server types

- [ ] **Step 5: Update originpool_mapper.go — marshal typed structs**

Add wire type and mapper function in `originpool_mapper.go`:

```go
type xcOriginPoolTLS struct {
	TLSConfig              *xcTLSConfig `json:"tls_config,omitempty"`
	SNI                    string       `json:"sni,omitempty"`
	VolterraTrustedCA      *struct{}    `json:"volterra_trusted_ca,omitempty"`
	TrustedCAURL           string       `json:"trusted_ca_url,omitempty"`
	DisableSNI             *struct{}    `json:"disable_sni,omitempty"`
	UseServerVerification  *struct{}    `json:"use_server_verification,omitempty"`
	SkipServerVerification *struct{}    `json:"skip_server_verification,omitempty"`
	NoMTLS                 *struct{}    `json:"no_mtls,omitempty"`
}

type xcTLSConfig struct {
	DefaultSecurity *struct{}            `json:"default_security,omitempty"`
	LowSecurity     *struct{}            `json:"low_security,omitempty"`
	MediumSecurity  *struct{}            `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity `json:"custom_security,omitempty"`
}
```

Update `mapOriginPoolSpec`:

```go
if spec.UseTLS != nil {
	out.UseTLS = mapOriginPoolTLS(spec.UseTLS)
}
if spec.NoTLS != nil {
	out.NoTLS = emptyObjectJSON
}
```

Add mapper function:

```go
func mapOriginPoolTLS(tls *v1alpha1.OriginPoolTLS) json.RawMessage {
	wire := xcOriginPoolTLS{
		SNI:                    tls.SNI,
		VolterraTrustedCA:      tls.VolterraTrustedCA,
		TrustedCAURL:           tls.TrustedCAURL,
		DisableSNI:             tls.DisableSNI,
		UseServerVerification:  tls.UseServerVerification,
		SkipServerVerification: tls.SkipServerVerification,
		NoMTLS:                 tls.NoMTLS,
	}
	wire.TLSConfig = &xcTLSConfig{
		DefaultSecurity: tls.DefaultSecurity,
		LowSecurity:     tls.LowSecurity,
		MediumSecurity:  tls.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(tls.CustomSecurity),
	}
	return marshalJSON(wire)
}
```

Update `mapNetworkChoice` to work with `*struct{}` instead of `*apiextensionsv1.JSON`:

```go
func mapNetworkChoice(inside, outside *struct{}) (json.RawMessage, json.RawMessage) {
	if inside != nil {
		return emptyObjectJSON, nil
	}
	if outside != nil {
		return nil, emptyObjectJSON
	}
	return nil, nil
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildOriginPool -v -count=1`
Expected: all origin pool mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/originpool_types.go internal/controller/originpool_mapper.go internal/controller/originpool_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(originpool): replace opaque JSON fields with typed structs"
```

---

### Task 4: AppFirewall — Type and Mapper Update (13 fields)

**Files:**
- Modify: `api/v1alpha1/appfirewall_types.go`
- Modify: `internal/controller/appfirewall_mapper.go`
- Modify: `internal/controller/appfirewall_mapper_test.go`

- [ ] **Step 1: Write the failing test — update appfirewall mapper test for typed fields**

In `internal/controller/appfirewall_mapper_test.go`, update the `sampleAppFirewall` function and tests:

```go
func sampleAppFirewall(name, namespace string) *v1alpha1.AppFirewall {
	return &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.AppFirewallSpec{
			XCNamespace: namespace,
			Blocking:    &struct{}{},
		},
	}
}

func TestBuildAppFirewallCreate_BasicFields(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "default")

	result := buildAppFirewallCreate(cr, "default")
	assert.Equal(t, "my-afw", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.JSONEq(t, "{}", string(result.Spec.Blocking))
	assert.Nil(t, result.Spec.Monitoring)
}

func TestBuildAppFirewallCreate_MultipleOneOfGroups(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-multi", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking:               &struct{}{},
			UseDefaultBlockingPage: &struct{}{},
			DefaultBotSetting:      &struct{}{},
			DefaultAnonymization:   &struct{}{},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.Blocking))
	assert.JSONEq(t, `{}`, string(result.Spec.UseDefaultBlockingPage))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultBotSetting))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultAnonymization))
	assert.Nil(t, result.Spec.Monitoring)
	assert.Nil(t, result.Spec.BlockingPage)
}

func TestBuildAppFirewallCreate_DetectionSettings(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-detect", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			DetectionSettings: &v1alpha1.DetectionSettings{
				EnableSuppression:     &struct{}{},
				EnableThreatCampaigns: &struct{}{},
				SignatureSelectionSetting: &v1alpha1.SignatureSelectionSetting{
					DefaultAttackTypeSettings: &struct{}{},
					OnlyHighAccuracySignatures: &struct{}{},
				},
			},
			Blocking: &struct{}{},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.NotNil(t, result.Spec.DetectionSettings)
	assert.Nil(t, result.Spec.DefaultDetectionSettings)

	var ds map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.DetectionSettings, &ds))
	_, hasEnable := ds["enable_suppression"]
	assert.True(t, hasEnable)
}

func TestBuildAppFirewallCreate_BotProtectionSetting(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-bot", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &struct{}{},
			BotProtectionSetting: &v1alpha1.BotProtectionSetting{
				MaliciousBotAction:  "BLOCK",
				SuspiciousBotAction: "FLAG",
				GoodBotAction:       "REPORT",
			},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{"malicious_bot_action":"BLOCK","suspicious_bot_action":"FLAG","good_bot_action":"REPORT"}`, string(result.Spec.BotProtectionSetting))
}

func TestBuildAppFirewallCreate_AllowedResponseCodes(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-codes", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking:             &struct{}{},
			AllowedResponseCodes: &v1alpha1.AllowedResponseCodes{ResponseCode: []int{200, 201, 204}},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{"response_code":[200,201,204]}`, string(result.Spec.AllowedResponseCodes))
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildAppFirewall -v -count=1 2>&1 | head -20`
Expected: compilation error

- [ ] **Step 3: Update appfirewall_types.go — replace JSON fields with typed structs**

```go
type AppFirewallSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// Detection — OneOf: DefaultDetectionSettings | DetectionSettings
	DefaultDetectionSettings *struct{}           `json:"defaultDetectionSettings,omitempty"`
	DetectionSettings        *DetectionSettings  `json:"detectionSettings,omitempty"`

	// Enforcement mode — OneOf: Monitoring | Blocking
	Monitoring *struct{} `json:"monitoring,omitempty"`
	Blocking   *struct{} `json:"blocking,omitempty"`

	// Blocking page — OneOf: UseDefaultBlockingPage | BlockingPage
	UseDefaultBlockingPage *struct{}     `json:"useDefaultBlockingPage,omitempty"`
	BlockingPage           *BlockingPage `json:"blockingPage,omitempty"`

	// Response codes — OneOf: AllowAllResponseCodes | AllowedResponseCodes
	AllowAllResponseCodes *struct{}             `json:"allowAllResponseCodes,omitempty"`
	AllowedResponseCodes  *AllowedResponseCodes `json:"allowedResponseCodes,omitempty"`

	// Bot setting — OneOf: DefaultBotSetting | BotProtectionSetting
	DefaultBotSetting    *struct{}             `json:"defaultBotSetting,omitempty"`
	BotProtectionSetting *BotProtectionSetting `json:"botProtectionSetting,omitempty"`

	// Anonymization — OneOf: DefaultAnonymization | DisableAnonymization | CustomAnonymization
	DefaultAnonymization *struct{}            `json:"defaultAnonymization,omitempty"`
	DisableAnonymization *struct{}            `json:"disableAnonymization,omitempty"`
	CustomAnonymization  *CustomAnonymization `json:"customAnonymization,omitempty"`
}

type DetectionSettings struct {
	SignatureSelectionSetting *SignatureSelectionSetting `json:"signatureSelectionSetting,omitempty"`
	EnableSuppression        *struct{}                  `json:"enableSuppression,omitempty"`
	DisableSuppression       *struct{}                  `json:"disableSuppression,omitempty"`
	EnableThreatCampaigns    *struct{}                  `json:"enableThreatCampaigns,omitempty"`
	DisableThreatCampaigns   *struct{}                  `json:"disableThreatCampaigns,omitempty"`
}

type SignatureSelectionSetting struct {
	DefaultAttackTypeSettings       *struct{}           `json:"defaultAttackTypeSettings,omitempty"`
	AttackTypeSettings              *AttackTypeSettings `json:"attackTypeSettings,omitempty"`
	HighMediumLowAccuracySignatures *struct{}           `json:"highMediumLowAccuracySignatures,omitempty"`
	HighMediumAccuracySignatures    *struct{}           `json:"highMediumAccuracySignatures,omitempty"`
	OnlyHighAccuracySignatures      *struct{}           `json:"onlyHighAccuracySignatures,omitempty"`
}

type AttackTypeSettings struct {
	DisabledAttackTypes []AttackType `json:"disabledAttackTypes,omitempty"`
}

type AttackType struct {
	Name string `json:"name"`
}

type BlockingPage struct {
	BlockingPage string `json:"blockingPage,omitempty"`
	ResponseCode string `json:"responseCode,omitempty"`
}

type AllowedResponseCodes struct {
	ResponseCode []int `json:"responseCode"`
}

type BotProtectionSetting struct {
	MaliciousBotAction  string `json:"maliciousBotAction,omitempty"`
	SuspiciousBotAction string `json:"suspiciousBotAction,omitempty"`
	GoodBotAction       string `json:"goodBotAction,omitempty"`
}

type CustomAnonymization struct {
	AnonymizationConfig []AnonymizationEntry `json:"anonymizationConfig,omitempty"`
	SpecificDomains     []string             `json:"specificDomains,omitempty"`
}

type AnonymizationEntry struct {
	HeaderName     string `json:"headerName,omitempty"`
	QueryParameter string `json:"queryParameter,omitempty"`
	CookieName     string `json:"cookieName,omitempty"`
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`

- [ ] **Step 5: Update appfirewall_mapper.go — add wire types and marshal logic**

Add wire types:

```go
type xcDetectionSettings struct {
	SignatureSelectionSetting *xcSignatureSelectionSetting `json:"signature_selection_setting,omitempty"`
	EnableSuppression        *struct{}                    `json:"enable_suppression,omitempty"`
	DisableSuppression       *struct{}                    `json:"disable_suppression,omitempty"`
	EnableThreatCampaigns    *struct{}                    `json:"enable_threat_campaigns,omitempty"`
	DisableThreatCampaigns   *struct{}                    `json:"disable_threat_campaigns,omitempty"`
}

type xcSignatureSelectionSetting struct {
	DefaultAttackTypeSettings       *struct{}              `json:"default_attack_type_settings,omitempty"`
	AttackTypeSettings              *xcAttackTypeSettings  `json:"attack_type_settings,omitempty"`
	HighMediumLowAccuracySignatures *struct{}              `json:"high_medium_low_accuracy_signatures,omitempty"`
	HighMediumAccuracySignatures    *struct{}              `json:"high_medium_accuracy_signatures,omitempty"`
	OnlyHighAccuracySignatures      *struct{}              `json:"only_high_accuracy_signatures,omitempty"`
}

type xcAttackTypeSettings struct {
	DisabledAttackTypes []xcAttackType `json:"disabled_attack_types,omitempty"`
}

type xcAttackType struct {
	Name string `json:"name"`
}

type xcBlockingPage struct {
	BlockingPage string `json:"blocking_page,omitempty"`
	ResponseCode string `json:"response_code,omitempty"`
}

type xcAllowedResponseCodes struct {
	ResponseCode []int `json:"response_code"`
}

type xcBotProtectionSetting struct {
	MaliciousBotAction  string `json:"malicious_bot_action,omitempty"`
	SuspiciousBotAction string `json:"suspicious_bot_action,omitempty"`
	GoodBotAction       string `json:"good_bot_action,omitempty"`
}

type xcCustomAnonymization struct {
	AnonymizationConfig []xcAnonymizationEntry `json:"anonymization_config,omitempty"`
	SpecificDomains     []string               `json:"specific_domains,omitempty"`
}

type xcAnonymizationEntry struct {
	HeaderName     string `json:"header_name,omitempty"`
	QueryParameter string `json:"query_parameter,omitempty"`
	CookieName     string `json:"cookie_name,omitempty"`
}
```

Update `mapAppFirewallSpec`:

```go
func mapAppFirewallSpec(spec *v1alpha1.AppFirewallSpec) xcclient.AppFirewallSpec {
	out := xcclient.AppFirewallSpec{}

	if spec.DefaultDetectionSettings != nil {
		out.DefaultDetectionSettings = emptyObjectJSON
	}
	if spec.DetectionSettings != nil {
		out.DetectionSettings = mapDetectionSettings(spec.DetectionSettings)
	}
	if spec.Monitoring != nil {
		out.Monitoring = emptyObjectJSON
	}
	if spec.Blocking != nil {
		out.Blocking = emptyObjectJSON
	}
	if spec.UseDefaultBlockingPage != nil {
		out.UseDefaultBlockingPage = emptyObjectJSON
	}
	if spec.BlockingPage != nil {
		out.BlockingPage = marshalJSON(xcBlockingPage{
			BlockingPage: spec.BlockingPage.BlockingPage,
			ResponseCode: spec.BlockingPage.ResponseCode,
		})
	}
	if spec.AllowAllResponseCodes != nil {
		out.AllowAllResponseCodes = emptyObjectJSON
	}
	if spec.AllowedResponseCodes != nil {
		out.AllowedResponseCodes = marshalJSON(xcAllowedResponseCodes{
			ResponseCode: spec.AllowedResponseCodes.ResponseCode,
		})
	}
	if spec.DefaultBotSetting != nil {
		out.DefaultBotSetting = emptyObjectJSON
	}
	if spec.BotProtectionSetting != nil {
		out.BotProtectionSetting = marshalJSON(xcBotProtectionSetting{
			MaliciousBotAction:  spec.BotProtectionSetting.MaliciousBotAction,
			SuspiciousBotAction: spec.BotProtectionSetting.SuspiciousBotAction,
			GoodBotAction:       spec.BotProtectionSetting.GoodBotAction,
		})
	}
	if spec.DefaultAnonymization != nil {
		out.DefaultAnonymization = emptyObjectJSON
	}
	if spec.DisableAnonymization != nil {
		out.DisableAnonymization = emptyObjectJSON
	}
	if spec.CustomAnonymization != nil {
		out.CustomAnonymization = mapCustomAnonymization(spec.CustomAnonymization)
	}

	return out
}

func mapDetectionSettings(ds *v1alpha1.DetectionSettings) json.RawMessage {
	wire := xcDetectionSettings{
		EnableSuppression:      ds.EnableSuppression,
		DisableSuppression:     ds.DisableSuppression,
		EnableThreatCampaigns:  ds.EnableThreatCampaigns,
		DisableThreatCampaigns: ds.DisableThreatCampaigns,
	}
	if ds.SignatureSelectionSetting != nil {
		wire.SignatureSelectionSetting = &xcSignatureSelectionSetting{
			DefaultAttackTypeSettings:       ds.SignatureSelectionSetting.DefaultAttackTypeSettings,
			HighMediumLowAccuracySignatures: ds.SignatureSelectionSetting.HighMediumLowAccuracySignatures,
			HighMediumAccuracySignatures:    ds.SignatureSelectionSetting.HighMediumAccuracySignatures,
			OnlyHighAccuracySignatures:      ds.SignatureSelectionSetting.OnlyHighAccuracySignatures,
		}
		if ds.SignatureSelectionSetting.AttackTypeSettings != nil {
			var disabled []xcAttackType
			for _, at := range ds.SignatureSelectionSetting.AttackTypeSettings.DisabledAttackTypes {
				disabled = append(disabled, xcAttackType{Name: at.Name})
			}
			wire.SignatureSelectionSetting.AttackTypeSettings = &xcAttackTypeSettings{
				DisabledAttackTypes: disabled,
			}
		}
	}
	return marshalJSON(wire)
}

func mapCustomAnonymization(ca *v1alpha1.CustomAnonymization) json.RawMessage {
	wire := xcCustomAnonymization{SpecificDomains: ca.SpecificDomains}
	for _, e := range ca.AnonymizationConfig {
		wire.AnonymizationConfig = append(wire.AnonymizationConfig, xcAnonymizationEntry{
			HeaderName:     e.HeaderName,
			QueryParameter: e.QueryParameter,
			CookieName:     e.CookieName,
		})
	}
	return marshalJSON(wire)
}
```

Remove the `"encoding/json"` import if no longer directly used, and remove the `apiextensionsv1` import.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildAppFirewall -v -count=1`
Expected: all appfirewall mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/appfirewall_types.go internal/controller/appfirewall_mapper.go internal/controller/appfirewall_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(appfirewall): replace opaque JSON fields with typed structs"
```

---

### Task 5: ServicePolicy — Type and Mapper Update (8 fields)

**Files:**
- Modify: `api/v1alpha1/servicepolicy_types.go`
- Modify: `internal/controller/servicepolicy_mapper.go`
- Modify: `internal/controller/servicepolicy_mapper_test.go`

- [ ] **Step 1: Write the failing test — update servicepolicy mapper test for typed fields**

In `internal/controller/servicepolicy_mapper_test.go`, update all tests that use `*apiextensionsv1.JSON`:

```go
func TestBuildServicePolicyCreate_BasicFields(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sp", Namespace: "default"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "default",
			AllowAllRequests: &struct{}{},
			AnyServer:        &struct{}{},
		},
	}
	result := buildServicePolicyCreate(cr, "default")
	assert.Equal(t, "my-sp", result.Metadata.Name)
	assert.JSONEq(t, `{}`, string(result.Spec.AllowAllRequests))
	assert.JSONEq(t, `{}`, string(result.Spec.AnyServer))
}

func TestBuildServicePolicyCreate_AllowList(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-allow", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			AllowList: &v1alpha1.PolicyAllowDenyList{
				Prefixes:    []string{"10.0.0.0/8"},
				CountryList: []string{"US", "GB"},
				DefaultActionNextPolicy: &struct{}{},
			},
			AnyServer: &struct{}{},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.NotNil(t, result.Spec.AllowList)
	assert.Nil(t, result.Spec.AllowAllRequests)

	var al map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.AllowList, &al))
	prefixes, ok := al["prefixes"].([]interface{})
	require.True(t, ok)
	assert.Len(t, prefixes, 1)
}

func TestBuildServicePolicyCreate_RuleList(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-rules", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace: "ns",
			RuleList: &v1alpha1.PolicyRuleList{
				Rules: []v1alpha1.PolicyRule{
					{
						Metadata: map[string]string{"name": "rule-1"},
						Spec: &v1alpha1.PolicyRuleSpec{
							Action:    "ALLOW",
							AnyClient: &struct{}{},
						},
					},
				},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.NotNil(t, result.Spec.RuleList)
}

func TestBuildServicePolicyCreate_ServerNameMatcher(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-snm", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &struct{}{},
			ServerNameMatcher: &v1alpha1.ServerNameMatcher{
				ExactValues: []string{"api.example.com"},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, `{"exact_values":["api.example.com"]}`, string(result.Spec.ServerNameMatcher))
}

func TestBuildServicePolicyCreate_ServerSelector(t *testing.T) {
	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sp-ss", Namespace: "ns"},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      "ns",
			AllowAllRequests: &struct{}{},
			ServerSelector: &v1alpha1.ServerSelector{
				Expressions: []string{"app in (web, api)"},
			},
		},
	}
	result := buildServicePolicyCreate(cr, "ns")
	assert.JSONEq(t, `{"expressions":["app in (web, api)"]}`, string(result.Spec.ServerSelector))
}
```

Also update tests that reference `apiextensionsv1.JSON` in DenyList, DenyAllRequests, and DesiredSpecJSON tests. Remove the `apiextensionsv1` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildServicePolicy -v -count=1 2>&1 | head -20`
Expected: compilation error

- [ ] **Step 3: Update servicepolicy_types.go — replace JSON fields with typed structs**

```go
type ServicePolicySpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string `json:"xcNamespace"`

	// Rule choice OneOf
	AllowAllRequests *struct{}            `json:"allowAllRequests,omitempty"`
	AllowList        *PolicyAllowDenyList `json:"allowList,omitempty"`
	DenyAllRequests  *struct{}            `json:"denyAllRequests,omitempty"`
	DenyList         *PolicyAllowDenyList `json:"denyList,omitempty"`
	RuleList         *PolicyRuleList      `json:"ruleList,omitempty"`

	// Server choice OneOf
	AnyServer         *struct{}          `json:"anyServer,omitempty"`
	ServerName        string             `json:"serverName,omitempty"`
	ServerNameMatcher *ServerNameMatcher  `json:"serverNameMatcher,omitempty"`
	ServerSelector    *ServerSelector     `json:"serverSelector,omitempty"`
}

type PolicyAllowDenyList struct {
	Prefixes                []string    `json:"prefixes,omitempty"`
	IPPrefixSet             []ObjectRef `json:"ipPrefixSet,omitempty"`
	ASNList                 *ASNList    `json:"asnList,omitempty"`
	ASNSet                  []ObjectRef `json:"asnSet,omitempty"`
	CountryList             []string    `json:"countryList,omitempty"`
	DefaultActionNextPolicy *struct{}   `json:"defaultActionNextPolicy,omitempty"`
	DefaultActionDeny       *struct{}   `json:"defaultActionDeny,omitempty"`
	DefaultActionAllow      *struct{}   `json:"defaultActionAllow,omitempty"`
}

type ASNList struct {
	ASNumbers []uint32 `json:"asNumbers"`
}

type PolicyRuleList struct {
	Rules []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Spec     *PolicyRuleSpec   `json:"spec,omitempty"`
}

type PolicyRuleSpec struct {
	Action         string          `json:"action,omitempty"`
	AnyClient      *struct{}       `json:"anyClient,omitempty"`
	ClientName     string          `json:"clientName,omitempty"`
	ClientSelector *LabelSelector  `json:"clientSelector,omitempty"`
	IPMatcher      *IPMatcher      `json:"ipMatcher,omitempty"`
	ASNMatcher     *ASNMatcher     `json:"asnMatcher,omitempty"`
	Path           *PathMatcher    `json:"path,omitempty"`
	Headers        []HeaderMatcher `json:"headers,omitempty"`
	HTTPMethod     *HTTPMethodMatcher `json:"httpMethod,omitempty"`
}

type ServerNameMatcher struct {
	ExactValues []string `json:"exactValues,omitempty"`
	RegexValues []string `json:"regexValues,omitempty"`
}

type ServerSelector struct {
	Expressions []string `json:"expressions,omitempty"`
}
```

Remove the `apiextensionsv1` import.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`

- [ ] **Step 5: Update servicepolicy_mapper.go — add wire types and marshal logic**

Add wire types and update `mapServicePolicySpec`. The pattern is the same: sentinel fields use `emptyObjectJSON`, complex types use `marshalJSON` with wire-format structs. See the appfirewall task for the full pattern. Key wire types needed:

```go
type xcPolicyAllowDenyList struct {
	Prefixes                []string       `json:"prefixes,omitempty"`
	IPPrefixSet             []xcObjectRef  `json:"ip_prefix_set,omitempty"`
	ASNList                 *xcASNList     `json:"asn_list,omitempty"`
	ASNSet                  []xcObjectRef  `json:"asn_set,omitempty"`
	CountryList             []string       `json:"country_list,omitempty"`
	DefaultActionNextPolicy *struct{}      `json:"default_action_next_policy,omitempty"`
	DefaultActionDeny       *struct{}      `json:"default_action_deny,omitempty"`
	DefaultActionAllow      *struct{}      `json:"default_action_allow,omitempty"`
}

type xcASNList struct {
	ASNumbers []uint32 `json:"as_numbers"`
}

type xcPolicyRuleList struct {
	Rules []xcPolicyRule `json:"rules"`
}

type xcPolicyRule struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Spec     *xcPolicyRuleSpec `json:"spec,omitempty"`
}

type xcPolicyRuleSpec struct {
	Action         string              `json:"action,omitempty"`
	AnyClient      *struct{}           `json:"any_client,omitempty"`
	ClientName     string              `json:"client_name,omitempty"`
	ClientSelector *xcLabelSelector    `json:"client_selector,omitempty"`
	IPMatcher      *xcIPMatcher        `json:"ip_matcher,omitempty"`
	ASNMatcher     *xcASNMatcher       `json:"asn_matcher,omitempty"`
	Path           *xcPathMatcher      `json:"path,omitempty"`
	Headers        []xcHeaderMatcher   `json:"headers,omitempty"`
	HTTPMethod     *xcHTTPMethodMatcher `json:"http_method,omitempty"`
}

type xcServerNameMatcher struct {
	ExactValues []string `json:"exact_values,omitempty"`
	RegexValues []string `json:"regex_values,omitempty"`
}

type xcServerSelector struct {
	Expressions []string `json:"expressions,omitempty"`
}
```

Update `mapServicePolicySpec`:

```go
func mapServicePolicySpec(spec *v1alpha1.ServicePolicySpec) xcclient.ServicePolicySpec {
	var out xcclient.ServicePolicySpec

	if spec.AllowAllRequests != nil {
		out.AllowAllRequests = emptyObjectJSON
	}
	if spec.AllowList != nil {
		out.AllowList = mapPolicyAllowDenyList(spec.AllowList)
	}
	if spec.DenyAllRequests != nil {
		out.DenyAllRequests = emptyObjectJSON
	}
	if spec.DenyList != nil {
		out.DenyList = mapPolicyAllowDenyList(spec.DenyList)
	}
	if spec.RuleList != nil {
		out.RuleList = mapPolicyRuleList(spec.RuleList)
	}
	if spec.AnyServer != nil {
		out.AnyServer = emptyObjectJSON
	}
	out.ServerName = spec.ServerName
	if spec.ServerNameMatcher != nil {
		out.ServerNameMatcher = marshalJSON(xcServerNameMatcher{
			ExactValues: spec.ServerNameMatcher.ExactValues,
			RegexValues: spec.ServerNameMatcher.RegexValues,
		})
	}
	if spec.ServerSelector != nil {
		out.ServerSelector = marshalJSON(xcServerSelector{
			Expressions: spec.ServerSelector.Expressions,
		})
	}

	return out
}
```

Add `mapPolicyAllowDenyList` and `mapPolicyRuleList` helper functions following the same pattern as `mapDetectionSettings` in the AppFirewall task. Remove `apiextensionsv1` import.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildServicePolicy -v -count=1`
Expected: all servicepolicy mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/servicepolicy_types.go internal/controller/servicepolicy_mapper.go internal/controller/servicepolicy_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(servicepolicy): replace opaque JSON fields with typed structs"
```

---

### Task 6: TCPLoadBalancer — Type and Mapper Update (7 fields)

**Files:**
- Modify: `api/v1alpha1/tcplb_types.go`
- Modify: `internal/controller/tcplb_mapper.go`
- Modify: `internal/controller/tcplb_mapper_test.go`

- [ ] **Step 1: Write the failing test — update tcplb mapper test for typed fields**

```go
func TestBuildTCPLoadBalancerCreate_TLSTCPAutoCert(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-tls", "ns")
	cr.Spec.TLSTCPAutoCert = &v1alpha1.TLSTCPAutoCert{
		NoMTLS: &struct{}{},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"no_mtls":{}}`, string(result.Spec.TLSTCPAutoCert))
	assert.Nil(t, result.Spec.TCP)
	assert.Nil(t, result.Spec.TLSTCP)
}

func TestBuildTCPLoadBalancerCreate_NoTLS(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-notls", "ns")
	cr.Spec.NoTLS = &struct{}{}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.TCP))
	assert.Nil(t, result.Spec.TLSTCP)
}

func TestBuildTCPLoadBalancerCreate_TLSParameters(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-tls-params", "ns")
	cr.Spec.TLSParameters = &v1alpha1.TLSParameters{
		DefaultSecurity: &struct{}{},
		NoMTLS:          &struct{}{},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.TLSTCP)
	var tls map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.TLSTCP, &tls))
	_, hasDefaultSecurity := tls["default_security"]
	assert.True(t, hasDefaultSecurity)
}

func TestBuildTCPLoadBalancerCreate_AdvertiseCustom(t *testing.T) {
	cr := sampleTCPLoadBalancer("tlb-adv", "ns")
	cr.Spec.AdvertiseCustom = &v1alpha1.AdvertiseCustom{
		AdvertiseWhere: []v1alpha1.AdvertiseWhere{
			{Port: 8080, Site: &v1alpha1.AdvertiseSite{
				Network: "SITE_NETWORK_INSIDE",
				Site:    v1alpha1.ObjectRef{Name: "my-site"},
			}},
		},
	}

	result := buildTCPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.AdvertiseCustom)
}
```

Remove `apiextensionsv1` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildTCPLoadBalancer -v -count=1 2>&1 | head -20`
Expected: compilation error

- [ ] **Step 3: Update tcplb_types.go — replace JSON fields with typed structs**

```go
type TCPLoadBalancerSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string      `json:"xcNamespace"`
	Domains     []string    `json:"domains"`
	ListenPort  uint32      `json:"listenPort"`
	OriginPools []RoutePool `json:"originPools"`

	// TLS OneOf
	NoTLS          *struct{}       `json:"noTLS,omitempty"`
	TLSParameters  *TLSParameters  `json:"tlsParameters,omitempty"`
	TLSTCPAutoCert *TLSTCPAutoCert `json:"tlsTCPAutoCert,omitempty"`

	// Advertise OneOf
	AdvertiseOnPublicDefaultVIP *struct{}         `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *AdvertiseOnPublic `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *AdvertiseCustom   `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *struct{}          `json:"doNotAdvertise,omitempty"`
}

type TLSParameters struct {
	TLSCertificates []TLSCertificateRef `json:"tlsCertificates,omitempty"`
	DefaultSecurity *struct{}           `json:"defaultSecurity,omitempty"`
	LowSecurity     *struct{}           `json:"lowSecurity,omitempty"`
	MediumSecurity  *struct{}           `json:"mediumSecurity,omitempty"`
	CustomSecurity  *CustomTLSSecurity  `json:"customSecurity,omitempty"`
	NoMTLS          *struct{}           `json:"noMTLS,omitempty"`
	UseMTLS         *UseMTLS            `json:"useMTLS,omitempty"`
}

type TLSTCPAutoCert struct {
	NoMTLS  *struct{} `json:"noMTLS,omitempty"`
	UseMTLS *UseMTLS  `json:"useMTLS,omitempty"`
}
```

Remove `apiextensionsv1` import.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`

- [ ] **Step 5: Update tcplb_mapper.go — add wire types and marshal logic**

Add wire types:

```go
type xcTLSParameters struct {
	TLSCertificates []xcTLSCertificateRef `json:"tls_certificates,omitempty"`
	DefaultSecurity *struct{}             `json:"default_security,omitempty"`
	LowSecurity     *struct{}             `json:"low_security,omitempty"`
	MediumSecurity  *struct{}             `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity  `json:"custom_security,omitempty"`
	NoMTLS          *struct{}             `json:"no_mtls,omitempty"`
	UseMTLS         *xcUseMTLS            `json:"use_mtls,omitempty"`
}

type xcTLSTCPAutoCert struct {
	NoMTLS  *struct{} `json:"no_mtls,omitempty"`
	UseMTLS *xcUseMTLS `json:"use_mtls,omitempty"`
}
```

Update `mapTCPLoadBalancerSpec`:

```go
// TLS OneOf
if spec.NoTLS != nil {
	out.TCP = emptyObjectJSON
}
if spec.TLSParameters != nil {
	out.TLSTCP = mapTLSParameters(spec.TLSParameters)
}
if spec.TLSTCPAutoCert != nil {
	out.TLSTCPAutoCert = marshalJSON(xcTLSTCPAutoCert{
		NoMTLS:  spec.TLSTCPAutoCert.NoMTLS,
		UseMTLS: mapXCUseMTLS(spec.TLSTCPAutoCert.UseMTLS),
	})
}

// Advertise OneOf
if spec.AdvertiseOnPublicDefaultVIP != nil {
	out.AdvertiseOnPublicDefaultVIP = emptyObjectJSON
}
if spec.AdvertiseOnPublic != nil {
	out.AdvertiseOnPublic = mapXCAdvertiseOnPublic(spec.AdvertiseOnPublic)
}
if spec.AdvertiseCustom != nil {
	out.AdvertiseCustom = mapXCAdvertiseCustom(spec.AdvertiseCustom)
}
if spec.DoNotAdvertise != nil {
	out.DoNotAdvertise = emptyObjectJSON
}
```

Add `mapTLSParameters` helper:

```go
func mapTLSParameters(p *v1alpha1.TLSParameters) json.RawMessage {
	return marshalJSON(xcTLSParameters{
		TLSCertificates: mapXCTLSCertificateRefs(p.TLSCertificates),
		DefaultSecurity: p.DefaultSecurity,
		LowSecurity:     p.LowSecurity,
		MediumSecurity:  p.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(p.CustomSecurity),
		NoMTLS:          p.NoMTLS,
		UseMTLS:         mapXCUseMTLS(p.UseMTLS),
	})
}
```

Remove `apiextensionsv1` import.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildTCPLoadBalancer -v -count=1`
Expected: all TCP LB mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/tcplb_types.go internal/controller/tcplb_mapper.go internal/controller/tcplb_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(tcplb): replace opaque JSON fields with typed structs"
```

---

### Task 7: HTTPLoadBalancer — Type and Mapper Update (30+ fields)

This is the largest CRD. Same pattern as previous tasks.

**Files:**
- Modify: `api/v1alpha1/httplb_types.go`
- Modify: `internal/controller/httplb_mapper.go`
- Modify: `internal/controller/httplb_mapper_test.go`

- [ ] **Step 1: Write the failing test — update httplb mapper test for typed fields**

Update `sampleHTTPLoadBalancer` and all tests:

```go
func sampleHTTPLoadBalancer(name, namespace string) *v1alpha1.HTTPLoadBalancer {
	return &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: namespace,
			Domains:     []string{"app.example.com"},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "pool1"}, Weight: uint32Ptr(1)},
			},
			AdvertiseOnPublicDefaultVIP: &struct{}{},
		},
	}
}

func TestBuildHTTPLoadBalancerCreate_TLSOneOf(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-tls", "ns")
	cr.Spec.HTTPS = &v1alpha1.HTTPSConfig{
		HTTPRedirect: true,
		AddHSTS:      true,
		DefaultSecurity: &struct{}{},
		NoMTLS:          &struct{}{},
		Port:            443,
	}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.HTTPS)

	var https map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.HTTPS, &https))
	assert.Equal(t, true, https["http_redirect"])
	assert.Equal(t, true, https["add_hsts"])
}

func TestBuildHTTPLoadBalancerCreate_HTTP(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-http", "ns")
	cr.Spec.HTTP = &v1alpha1.HTTPConfig{
		DNSVolterraManaged: false,
		Port:               80,
	}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"dns_volterra_managed":false,"port":80}`, string(result.Spec.HTTP))
}

func TestBuildHTTPLoadBalancerCreate_CookieStickiness(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-cookie", "ns")
	cr.Spec.CookieStickiness = &v1alpha1.CookieStickinessConfig{
		Name: "session",
		Path: "/",
		TTL:  3600,
	}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"name":"session","path":"/","ttl":3600}`, string(result.Spec.CookieStickiness))
}

func TestBuildHTTPLoadBalancerCreate_DisableOptions(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-disabled", "ns")
	cr.Spec.DisableWAF = &struct{}{}
	cr.Spec.DisableBotDefense = &struct{}{}
	cr.Spec.DisableAPIDiscovery = &struct{}{}
	cr.Spec.NoChallenge = &struct{}{}
	cr.Spec.RoundRobin = &struct{}{}
	cr.Spec.NoServicePolicies = &struct{}{}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.DisableWAF))
	assert.JSONEq(t, `{}`, string(result.Spec.DisableBotDefense))
	assert.JSONEq(t, `{}`, string(result.Spec.DisableAPIDiscovery))
	assert.JSONEq(t, `{}`, string(result.Spec.NoChallenge))
	assert.JSONEq(t, `{}`, string(result.Spec.RoundRobin))
	assert.JSONEq(t, `{}`, string(result.Spec.NoServicePolicies))
}

func TestBuildHTTPLoadBalancerCreate_ActiveServicePolicies(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-sp", "ns")
	cr.Spec.ActiveServicePolicies = &v1alpha1.ActiveServicePoliciesConfig{
		Policies: []v1alpha1.ObjectRef{{Name: "policy1", Namespace: "ns"}},
	}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{"policies":[{"name":"policy1","namespace":"ns"}]}`, string(result.Spec.ActiveServicePolicies))
}

func TestBuildHTTPLoadBalancerCreate_UserIDClientIP(t *testing.T) {
	cr := sampleHTTPLoadBalancer("hlb-uid", "ns")
	cr.Spec.UserIDClientIP = &struct{}{}

	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.UserIDClientIP))
}
```

Remove `apiextensionsv1` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildHTTPLoadBalancer -v -count=1 2>&1 | head -20`
Expected: compilation error

- [ ] **Step 3: Update httplb_types.go — replace JSON fields with typed structs**

```go
type HTTPLoadBalancerSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace       string                 `json:"xcNamespace"`
	Domains           []string               `json:"domains"`
	DefaultRoutePools []RoutePool            `json:"defaultRoutePools"`
	Routes            []apiextensionsv1.JSON `json:"routes,omitempty"`

	// TLS OneOf
	HTTP          *HTTPConfig          `json:"http,omitempty"`
	HTTPS         *HTTPSConfig         `json:"https,omitempty"`
	HTTPSAutoCert *HTTPSAutoCertConfig `json:"httpsAutoCert,omitempty"`

	// WAF OneOf
	DisableWAF  *struct{}  `json:"disableWAF,omitempty"`
	AppFirewall *ObjectRef `json:"appFirewall,omitempty"`

	// Bot defense OneOf
	DisableBotDefense *struct{}         `json:"disableBotDefense,omitempty"`
	BotDefense        *BotDefenseConfig `json:"botDefense,omitempty"`

	// API discovery OneOf
	DisableAPIDiscovery *struct{}                  `json:"disableAPIDiscovery,omitempty"`
	EnableAPIDiscovery  *EnableAPIDiscoveryConfig  `json:"enableAPIDiscovery,omitempty"`

	// IP reputation OneOf
	DisableIPReputation *struct{}                  `json:"disableIPReputation,omitempty"`
	EnableIPReputation  *EnableIPReputationConfig  `json:"enableIPReputation,omitempty"`

	// Rate limit OneOf
	DisableRateLimit *struct{}        `json:"disableRateLimit,omitempty"`
	RateLimit        *RateLimitConfig `json:"rateLimit,omitempty"`

	// Challenge OneOf
	NoChallenge          *struct{}                    `json:"noChallenge,omitempty"`
	JSChallenge          *JSChallengeConfig           `json:"jsChallenge,omitempty"`
	CaptchaChallenge     *CaptchaChallengeConfig      `json:"captchaChallenge,omitempty"`
	PolicyBasedChallenge *PolicyBasedChallengeConfig  `json:"policyBasedChallenge,omitempty"`

	// LB algorithm OneOf
	RoundRobin         *struct{}                `json:"roundRobin,omitempty"`
	LeastActive        *struct{}                `json:"leastActive,omitempty"`
	Random             *struct{}                `json:"random,omitempty"`
	SourceIPStickiness *struct{}                `json:"sourceIPStickiness,omitempty"`
	CookieStickiness   *CookieStickinessConfig  `json:"cookieStickiness,omitempty"`
	RingHash           *RingHashConfig          `json:"ringHash,omitempty"`

	// Advertise OneOf
	AdvertiseOnPublicDefaultVIP *struct{}          `json:"advertiseOnPublicDefaultVIP,omitempty"`
	AdvertiseOnPublic           *AdvertiseOnPublic `json:"advertiseOnPublic,omitempty"`
	AdvertiseCustom             *AdvertiseCustom   `json:"advertiseCustom,omitempty"`
	DoNotAdvertise              *struct{}          `json:"doNotAdvertise,omitempty"`

	// Service policies OneOf
	ServicePoliciesFromNamespace *struct{}                      `json:"servicePoliciesFromNamespace,omitempty"`
	ActiveServicePolicies        *ActiveServicePoliciesConfig   `json:"activeServicePolicies,omitempty"`
	NoServicePolicies            *struct{}                      `json:"noServicePolicies,omitempty"`

	// User ID OneOf
	UserIDClientIP *struct{} `json:"userIDClientIP,omitempty"`
}
```

Add the HTTP LB-specific types (these go in `httplb_types.go` since they're not shared):

```go
type HTTPConfig struct {
	DNSVolterraManaged bool   `json:"dnsVolterraManaged,omitempty"`
	Port               uint32 `json:"port,omitempty"`
}

type HTTPSConfig struct {
	HTTPRedirect    bool                `json:"httpRedirect,omitempty"`
	AddHSTS         bool                `json:"addHSTS,omitempty"`
	TLSCertificates []TLSCertificateRef `json:"tlsCertificates,omitempty"`
	DefaultSecurity *struct{}           `json:"defaultSecurity,omitempty"`
	LowSecurity     *struct{}           `json:"lowSecurity,omitempty"`
	MediumSecurity  *struct{}           `json:"mediumSecurity,omitempty"`
	CustomSecurity  *CustomTLSSecurity  `json:"customSecurity,omitempty"`
	NoMTLS          *struct{}           `json:"noMTLS,omitempty"`
	UseMTLS         *UseMTLS            `json:"useMTLS,omitempty"`
	Port            uint32              `json:"port,omitempty"`
}

type HTTPSAutoCertConfig struct {
	HTTPRedirect bool      `json:"httpRedirect,omitempty"`
	AddHSTS      bool      `json:"addHSTS,omitempty"`
	NoMTLS       *struct{} `json:"noMTLS,omitempty"`
	UseMTLS      *UseMTLS  `json:"useMTLS,omitempty"`
	Port         uint32    `json:"port,omitempty"`
}

type BotDefenseConfig struct {
	RegionalEndpoint string            `json:"regionalEndpoint,omitempty"`
	Policy           *BotDefensePolicy `json:"policy,omitempty"`
	Timeout          uint32            `json:"timeout,omitempty"`
}

type BotDefensePolicy struct {
	ProtectedAppEndpoints []ProtectedAppEndpoint `json:"protectedAppEndpoints,omitempty"`
	JSInsertionRules      *JSInsertionRules      `json:"jsInsertionRules,omitempty"`
	JsDownloadPath        string                 `json:"jsDownloadPath,omitempty"`
	DisableMobileSDK      *struct{}              `json:"disableMobileSDK,omitempty"`
}

type ProtectedAppEndpoint struct {
	Metadata         map[string]string      `json:"metadata,omitempty"`
	HTTPMethods      []string               `json:"httpMethods,omitempty"`
	Path             PathMatcher            `json:"path"`
	Flow             string                 `json:"flow,omitempty"`
	Protocol         string                 `json:"protocol,omitempty"`
	AnyDomain        *struct{}              `json:"anyDomain,omitempty"`
	Domain           string                 `json:"domain,omitempty"`
	MitigationAction map[string]interface{} `json:"mitigationAction,omitempty"`
}

type JSInsertionRules struct {
	Rules []JSInsertionRule `json:"rules,omitempty"`
}

type JSInsertionRule struct {
	ExcludedPaths []PathMatcher `json:"excludedPaths,omitempty"`
}

type EnableAPIDiscoveryConfig struct {
	EnableLearnFromRedirectTraffic  *struct{}                    `json:"enableLearnFromRedirectTraffic,omitempty"`
	DisableLearnFromRedirectTraffic *struct{}                    `json:"disableLearnFromRedirectTraffic,omitempty"`
	DefaultAPIAuthDiscovery         *struct{}                    `json:"defaultAPIAuthDiscovery,omitempty"`
	APICrawler                      *ObjectRef                   `json:"apiCrawler,omitempty"`
	APIDiscoveryFromCodeScan        *ObjectRef                   `json:"apiDiscoveryFromCodeScan,omitempty"`
	DiscoveredAPISettings           *DiscoveredAPISettings       `json:"discoveredAPISettings,omitempty"`
	SensitiveDataDetectionRules     *SensitiveDataDetectionRules `json:"sensitiveDataDetectionRules,omitempty"`
}

type DiscoveredAPISettings struct {
	PurgeByDurationDays uint32 `json:"purgeByDurationDays,omitempty"`
}

type SensitiveDataDetectionRules struct {
	SensitiveDataTypes []SensitiveDataType `json:"sensitiveDataTypes,omitempty"`
}

type SensitiveDataType struct {
	Type string `json:"type"`
}

type EnableIPReputationConfig struct {
	IPThreatCategories []string `json:"ipThreatCategories,omitempty"`
}

type RateLimitConfig struct {
	RateLimiter     *RateLimiterInline `json:"rateLimiter,omitempty"`
	NoIPAllowedList *struct{}          `json:"noIPAllowedList,omitempty"`
	IPAllowedList   []ObjectRef        `json:"ipAllowedList,omitempty"`
	NoPolicies      *struct{}          `json:"noPolicies,omitempty"`
	Policies        *RateLimitPolicies `json:"policies,omitempty"`
}

type RateLimiterInline struct {
	TotalNumber     uint32 `json:"totalNumber"`
	Unit            string `json:"unit"`
	BurstMultiplier uint32 `json:"burstMultiplier"`
}

type RateLimitPolicies struct {
	Policies []ObjectRef `json:"policies,omitempty"`
}

type JSChallengeConfig struct {
	JSScriptDelay uint32 `json:"jsScriptDelay,omitempty"`
	CookieExpiry  uint32 `json:"cookieExpiry,omitempty"`
	CustomPage    string `json:"customPage,omitempty"`
}

type CaptchaChallengeConfig struct {
	Expiry     uint32 `json:"expiry,omitempty"`
	CustomPage string `json:"customPage,omitempty"`
}

type PolicyBasedChallengeConfig struct {
	DefaultJSChallengeParameters     *JSChallengeConfig      `json:"defaultJSChallengeParameters,omitempty"`
	DefaultCaptchaChallengeParameters *CaptchaChallengeConfig `json:"defaultCaptchaChallengeParameters,omitempty"`
	DefaultMitigationSettings        *struct{}               `json:"defaultMitigationSettings,omitempty"`
	AlwaysEnableJSChallenge          *struct{}               `json:"alwaysEnableJSChallenge,omitempty"`
	AlwaysEnableCaptcha              *struct{}               `json:"alwaysEnableCaptcha,omitempty"`
	NoChallenge                      *struct{}               `json:"noChallenge,omitempty"`
	MaliciousUserMitigationBypass    *struct{}               `json:"maliciousUserMitigationBypass,omitempty"`
	RuleList                         *ChallengeRuleList      `json:"ruleList,omitempty"`
	TemporaryBlockingParameters      *TemporaryBlockingParams `json:"temporaryBlockingParameters,omitempty"`
}

type ChallengeRuleList struct {
	Rules []ChallengeRule `json:"rules,omitempty"`
}

type ChallengeRule struct {
	Metadata map[string]string  `json:"metadata,omitempty"`
	Spec     *ChallengeRuleSpec `json:"spec,omitempty"`
}

type ChallengeRuleSpec struct {
	ChallengeAction string `json:"challengeAction,omitempty"`
}

type TemporaryBlockingParams struct {
	Duration uint32 `json:"duration,omitempty"`
}

type CookieStickinessConfig struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	TTL  uint32 `json:"ttl,omitempty"`
}

type RingHashConfig struct {
	HashPolicy []HashPolicy `json:"hashPolicy,omitempty"`
}

type HashPolicy struct {
	HeaderName string           `json:"headerName,omitempty"`
	Cookie     *CookieForHashing `json:"cookie,omitempty"`
	SourceIP   *struct{}        `json:"sourceIP,omitempty"`
	Terminal   bool             `json:"terminal,omitempty"`
}

type CookieForHashing struct {
	Name string `json:"name"`
	TTL  uint32 `json:"ttl,omitempty"`
	Path string `json:"path,omitempty"`
}

type ActiveServicePoliciesConfig struct {
	Policies []ObjectRef `json:"policies"`
}
```

Keep the `apiextensionsv1` import for the `Routes` field which stays as `[]apiextensionsv1.JSON`.

- [ ] **Step 4: Run make generate to update deepcopy**

Run: `make generate`

- [ ] **Step 5: Update httplb_mapper.go — add wire types and marshal logic**

This is the largest mapper update. Add wire types for each complex field and update `mapHTTPLoadBalancerSpec`. The pattern is identical to previous tasks:
- Sentinel `*struct{}` → `emptyObjectJSON`
- Complex typed fields → `marshalJSON(xcWireType{...})`

Key wire types needed (in `httplb_mapper.go`):

```go
type xcHTTPConfig struct {
	DNSVolterraManaged bool   `json:"dns_volterra_managed,omitempty"`
	Port               uint32 `json:"port,omitempty"`
}

type xcHTTPSConfig struct {
	HTTPRedirect    bool                  `json:"http_redirect,omitempty"`
	AddHSTS         bool                  `json:"add_hsts,omitempty"`
	TLSCertificates []xcTLSCertificateRef `json:"tls_certificates,omitempty"`
	DefaultSecurity *struct{}             `json:"default_security,omitempty"`
	LowSecurity     *struct{}             `json:"low_security,omitempty"`
	MediumSecurity  *struct{}             `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity  `json:"custom_security,omitempty"`
	NoMTLS          *struct{}             `json:"no_mtls,omitempty"`
	UseMTLS         *xcUseMTLS            `json:"use_mtls,omitempty"`
	Port            uint32                `json:"port,omitempty"`
}

type xcHTTPSAutoCertConfig struct {
	HTTPRedirect bool      `json:"http_redirect,omitempty"`
	AddHSTS      bool      `json:"add_hsts,omitempty"`
	NoMTLS       *struct{} `json:"no_mtls,omitempty"`
	UseMTLS      *xcUseMTLS `json:"use_mtls,omitempty"`
	Port         uint32    `json:"port,omitempty"`
}

type xcCookieStickinessConfig struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	TTL  uint32 `json:"ttl,omitempty"`
}

type xcActiveServicePoliciesConfig struct {
	Policies []xcObjectRef `json:"policies"`
}
```

Wire types for remaining complex fields:

```go
type xcBotDefenseConfig struct {
	RegionalEndpoint string              `json:"regional_endpoint,omitempty"`
	Policy           *xcBotDefensePolicy `json:"policy,omitempty"`
	Timeout          uint32              `json:"timeout,omitempty"`
}

type xcBotDefensePolicy struct {
	ProtectedAppEndpoints []xcProtectedAppEndpoint `json:"protected_app_endpoints,omitempty"`
	JSInsertionRules      *xcJSInsertionRules      `json:"js_insertion_rules,omitempty"`
	JsDownloadPath        string                   `json:"js_download_path,omitempty"`
	DisableMobileSDK      *struct{}                `json:"disable_mobile_sdk,omitempty"`
}

type xcProtectedAppEndpoint struct {
	Metadata         map[string]string      `json:"metadata,omitempty"`
	HTTPMethods      []string               `json:"http_methods,omitempty"`
	Path             xcPathMatcher          `json:"path"`
	Flow             string                 `json:"flow,omitempty"`
	Protocol         string                 `json:"protocol,omitempty"`
	AnyDomain        *struct{}              `json:"any_domain,omitempty"`
	Domain           string                 `json:"domain,omitempty"`
	MitigationAction map[string]interface{} `json:"mitigation_action,omitempty"`
}

type xcJSInsertionRules struct {
	Rules []xcJSInsertionRule `json:"rules,omitempty"`
}

type xcJSInsertionRule struct {
	ExcludedPaths []xcPathMatcher `json:"excluded_paths,omitempty"`
}

type xcEnableAPIDiscoveryConfig struct {
	EnableLearnFromRedirectTraffic  *struct{}                        `json:"enable_learn_from_redirect_traffic,omitempty"`
	DisableLearnFromRedirectTraffic *struct{}                        `json:"disable_learn_from_redirect_traffic,omitempty"`
	DefaultAPIAuthDiscovery         *struct{}                        `json:"default_api_auth_discovery,omitempty"`
	APICrawler                      *xcObjectRef                     `json:"api_crawler,omitempty"`
	APIDiscoveryFromCodeScan        *xcObjectRef                     `json:"api_discovery_from_code_scan,omitempty"`
	DiscoveredAPISettings           *xcDiscoveredAPISettings         `json:"discovered_api_settings,omitempty"`
	SensitiveDataDetectionRules     *xcSensitiveDataDetectionRules   `json:"sensitive_data_detection_rules,omitempty"`
}

type xcDiscoveredAPISettings struct {
	PurgeByDurationDays uint32 `json:"purge_duration_days,omitempty"`
}

type xcSensitiveDataDetectionRules struct {
	SensitiveDataTypes []xcSensitiveDataType `json:"sensitive_data_types,omitempty"`
}

type xcSensitiveDataType struct {
	Type string `json:"type"`
}

type xcEnableIPReputationConfig struct {
	IPThreatCategories []string `json:"ip_threat_categories,omitempty"`
}

type xcRateLimitConfig struct {
	RateLimiter     *xcRateLimiterInline `json:"rate_limiter,omitempty"`
	NoIPAllowedList *struct{}            `json:"no_ip_allowed_list,omitempty"`
	IPAllowedList   []xcObjectRef        `json:"ip_allowed_list,omitempty"`
	NoPolicies      *struct{}            `json:"no_policies,omitempty"`
	Policies        *xcRateLimitPolicies `json:"policies,omitempty"`
}

type xcRateLimiterInline struct {
	TotalNumber     uint32 `json:"total_number"`
	Unit            string `json:"unit"`
	BurstMultiplier uint32 `json:"burst_multiplier"`
}

type xcRateLimitPolicies struct {
	Policies []xcObjectRef `json:"policies,omitempty"`
}

type xcJSChallengeConfig struct {
	JSScriptDelay uint32 `json:"js_script_delay,omitempty"`
	CookieExpiry  uint32 `json:"cookie_expiry,omitempty"`
	CustomPage    string `json:"custom_page,omitempty"`
}

type xcCaptchaChallengeConfig struct {
	Expiry     uint32 `json:"expiry,omitempty"`
	CustomPage string `json:"custom_page,omitempty"`
}

type xcPolicyBasedChallengeConfig struct {
	DefaultJSChallengeParameters      *xcJSChallengeConfig      `json:"default_js_challenge_parameters,omitempty"`
	DefaultCaptchaChallengeParameters *xcCaptchaChallengeConfig  `json:"default_captcha_challenge_parameters,omitempty"`
	DefaultMitigationSettings         *struct{}                  `json:"default_mitigation_settings,omitempty"`
	AlwaysEnableJSChallenge           *struct{}                  `json:"always_enable_js_challenge,omitempty"`
	AlwaysEnableCaptcha               *struct{}                  `json:"always_enable_captcha,omitempty"`
	NoChallenge                       *struct{}                  `json:"no_challenge,omitempty"`
	MaliciousUserMitigationBypass     *struct{}                  `json:"malicious_user_mitigation_bypass,omitempty"`
	RuleList                          *xcChallengeRuleList       `json:"rule_list,omitempty"`
	TemporaryBlockingParameters       *xcTemporaryBlockingParams `json:"temporary_blocking_parameters,omitempty"`
}

type xcChallengeRuleList struct {
	Rules []xcChallengeRule `json:"rules,omitempty"`
}

type xcChallengeRule struct {
	Metadata map[string]string   `json:"metadata,omitempty"`
	Spec     *xcChallengeRuleSpec `json:"spec,omitempty"`
}

type xcChallengeRuleSpec struct {
	ChallengeAction string `json:"challenge_action,omitempty"`
}

type xcTemporaryBlockingParams struct {
	Duration uint32 `json:"duration,omitempty"`
}

type xcRingHashConfig struct {
	HashPolicy []xcHashPolicy `json:"hash_policy,omitempty"`
}

type xcHashPolicy struct {
	HeaderName string             `json:"header_name,omitempty"`
	Cookie     *xcCookieForHashing `json:"cookie,omitempty"`
	SourceIP   *struct{}          `json:"source_ip,omitempty"`
	Terminal   bool               `json:"terminal,omitempty"`
}

type xcCookieForHashing struct {
	Name string `json:"name"`
	TTL  uint32 `json:"ttl,omitempty"`
	Path string `json:"path,omitempty"`
}
```

Update `mapHTTPLoadBalancerSpec`:

```go
func mapHTTPLoadBalancerSpec(spec *v1alpha1.HTTPLoadBalancerSpec) xcclient.HTTPLoadBalancerSpec {
	var out xcclient.HTTPLoadBalancerSpec
	out.Domains = spec.Domains

	for i := range spec.DefaultRoutePools {
		out.DefaultRoutePools = append(out.DefaultRoutePools, mapRoutePool(&spec.DefaultRoutePools[i]))
	}

	if len(spec.Routes) > 0 {
		routesJSON, _ := json.Marshal(spec.Routes)
		out.Routes = routesJSON
	}

	// TLS OneOf
	if spec.HTTP != nil {
		out.HTTP = marshalJSON(xcHTTPConfig{
			DNSVolterraManaged: spec.HTTP.DNSVolterraManaged,
			Port:               spec.HTTP.Port,
		})
	}
	if spec.HTTPS != nil {
		out.HTTPS = mapHTTPSConfig(spec.HTTPS)
	}
	if spec.HTTPSAutoCert != nil {
		out.HTTPSAutoCert = marshalJSON(xcHTTPSAutoCertConfig{
			HTTPRedirect: spec.HTTPSAutoCert.HTTPRedirect,
			AddHSTS:      spec.HTTPSAutoCert.AddHSTS,
			NoMTLS:       spec.HTTPSAutoCert.NoMTLS,
			UseMTLS:      mapXCUseMTLS(spec.HTTPSAutoCert.UseMTLS),
			Port:         spec.HTTPSAutoCert.Port,
		})
	}

	// WAF OneOf
	if spec.DisableWAF != nil {
		out.DisableWAF = emptyObjectJSON
	}
	if spec.AppFirewall != nil {
		out.AppFirewall = mapObjectRefPtr(spec.AppFirewall)
	}

	// Bot defense OneOf
	if spec.DisableBotDefense != nil {
		out.DisableBotDefense = emptyObjectJSON
	}
	if spec.BotDefense != nil {
		out.BotDefense = mapBotDefenseConfig(spec.BotDefense)
	}

	// API discovery OneOf
	if spec.DisableAPIDiscovery != nil {
		out.DisableAPIDiscovery = emptyObjectJSON
	}
	if spec.EnableAPIDiscovery != nil {
		out.EnableAPIDiscovery = mapEnableAPIDiscoveryConfig(spec.EnableAPIDiscovery)
	}

	// IP reputation OneOf
	if spec.DisableIPReputation != nil {
		out.DisableIPReputation = emptyObjectJSON
	}
	if spec.EnableIPReputation != nil {
		out.EnableIPReputation = marshalJSON(xcEnableIPReputationConfig{
			IPThreatCategories: spec.EnableIPReputation.IPThreatCategories,
		})
	}

	// Rate limit OneOf
	if spec.DisableRateLimit != nil {
		out.DisableRateLimit = emptyObjectJSON
	}
	if spec.RateLimit != nil {
		out.RateLimit = mapRateLimitConfig(spec.RateLimit)
	}

	// Challenge OneOf
	if spec.NoChallenge != nil {
		out.NoChallenge = emptyObjectJSON
	}
	if spec.JSChallenge != nil {
		out.JSChallenge = marshalJSON(xcJSChallengeConfig{
			JSScriptDelay: spec.JSChallenge.JSScriptDelay,
			CookieExpiry:  spec.JSChallenge.CookieExpiry,
			CustomPage:    spec.JSChallenge.CustomPage,
		})
	}
	if spec.CaptchaChallenge != nil {
		out.CaptchaChallenge = marshalJSON(xcCaptchaChallengeConfig{
			Expiry:     spec.CaptchaChallenge.Expiry,
			CustomPage: spec.CaptchaChallenge.CustomPage,
		})
	}
	if spec.PolicyBasedChallenge != nil {
		out.PolicyBasedChallenge = mapPolicyBasedChallengeConfig(spec.PolicyBasedChallenge)
	}

	// LB algorithm OneOf
	if spec.RoundRobin != nil { out.RoundRobin = emptyObjectJSON }
	if spec.LeastActive != nil { out.LeastActive = emptyObjectJSON }
	if spec.Random != nil { out.Random = emptyObjectJSON }
	if spec.SourceIPStickiness != nil { out.SourceIPStickiness = emptyObjectJSON }
	if spec.CookieStickiness != nil {
		out.CookieStickiness = marshalJSON(xcCookieStickinessConfig{
			Name: spec.CookieStickiness.Name,
			Path: spec.CookieStickiness.Path,
			TTL:  spec.CookieStickiness.TTL,
		})
	}
	if spec.RingHash != nil {
		out.RingHash = mapRingHash(spec.RingHash)
	}

	// Advertise OneOf
	if spec.AdvertiseOnPublicDefaultVIP != nil { out.AdvertiseOnPublicDefaultVIP = emptyObjectJSON }
	if spec.AdvertiseOnPublic != nil { out.AdvertiseOnPublic = mapXCAdvertiseOnPublic(spec.AdvertiseOnPublic) }
	if spec.AdvertiseCustom != nil { out.AdvertiseCustom = mapXCAdvertiseCustom(spec.AdvertiseCustom) }
	if spec.DoNotAdvertise != nil { out.DoNotAdvertise = emptyObjectJSON }

	// Service policies OneOf
	if spec.ServicePoliciesFromNamespace != nil { out.ServicePoliciesFromNamespace = emptyObjectJSON }
	if spec.ActiveServicePolicies != nil {
		out.ActiveServicePolicies = marshalJSON(xcActiveServicePoliciesConfig{
			Policies: mapXCObjectRefs(spec.ActiveServicePolicies.Policies),
		})
	}
	if spec.NoServicePolicies != nil { out.NoServicePolicies = emptyObjectJSON }

	// User ID OneOf
	if spec.UserIDClientIP != nil { out.UserIDClientIP = emptyObjectJSON }

	return out
}
```

Add mapper helper functions:

```go
func mapHTTPSConfig(h *v1alpha1.HTTPSConfig) json.RawMessage {
	return marshalJSON(xcHTTPSConfig{
		HTTPRedirect:    h.HTTPRedirect,
		AddHSTS:         h.AddHSTS,
		TLSCertificates: mapXCTLSCertificateRefs(h.TLSCertificates),
		DefaultSecurity: h.DefaultSecurity,
		LowSecurity:     h.LowSecurity,
		MediumSecurity:  h.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(h.CustomSecurity),
		NoMTLS:          h.NoMTLS,
		UseMTLS:         mapXCUseMTLS(h.UseMTLS),
		Port:            h.Port,
	})
}

func mapBotDefenseConfig(bd *v1alpha1.BotDefenseConfig) json.RawMessage {
	wire := xcBotDefenseConfig{
		RegionalEndpoint: bd.RegionalEndpoint,
		Timeout:          bd.Timeout,
	}
	if bd.Policy != nil {
		p := &xcBotDefensePolicy{
			JsDownloadPath:   bd.Policy.JsDownloadPath,
			DisableMobileSDK: bd.Policy.DisableMobileSDK,
		}
		for _, ep := range bd.Policy.ProtectedAppEndpoints {
			p.ProtectedAppEndpoints = append(p.ProtectedAppEndpoints, xcProtectedAppEndpoint{
				Metadata:         ep.Metadata,
				HTTPMethods:      ep.HTTPMethods,
				Path:             mapXCPathMatcher(ep.Path),
				Flow:             ep.Flow,
				Protocol:         ep.Protocol,
				AnyDomain:        ep.AnyDomain,
				Domain:           ep.Domain,
				MitigationAction: ep.MitigationAction,
			})
		}
		if bd.Policy.JSInsertionRules != nil {
			jsr := &xcJSInsertionRules{}
			for _, r := range bd.Policy.JSInsertionRules.Rules {
				xr := xcJSInsertionRule{}
				for _, ep := range r.ExcludedPaths {
					xr.ExcludedPaths = append(xr.ExcludedPaths, mapXCPathMatcher(ep))
				}
				jsr.Rules = append(jsr.Rules, xr)
			}
			p.JSInsertionRules = jsr
		}
		wire.Policy = p
	}
	return marshalJSON(wire)
}

func mapEnableAPIDiscoveryConfig(ad *v1alpha1.EnableAPIDiscoveryConfig) json.RawMessage {
	wire := xcEnableAPIDiscoveryConfig{
		EnableLearnFromRedirectTraffic:  ad.EnableLearnFromRedirectTraffic,
		DisableLearnFromRedirectTraffic: ad.DisableLearnFromRedirectTraffic,
		DefaultAPIAuthDiscovery:         ad.DefaultAPIAuthDiscovery,
		APICrawler:                      mapXCObjectRef(ad.APICrawler),
		APIDiscoveryFromCodeScan:        mapXCObjectRef(ad.APIDiscoveryFromCodeScan),
	}
	if ad.DiscoveredAPISettings != nil {
		wire.DiscoveredAPISettings = &xcDiscoveredAPISettings{
			PurgeByDurationDays: ad.DiscoveredAPISettings.PurgeByDurationDays,
		}
	}
	if ad.SensitiveDataDetectionRules != nil {
		sdr := &xcSensitiveDataDetectionRules{}
		for _, sdt := range ad.SensitiveDataDetectionRules.SensitiveDataTypes {
			sdr.SensitiveDataTypes = append(sdr.SensitiveDataTypes, xcSensitiveDataType{Type: sdt.Type})
		}
		wire.SensitiveDataDetectionRules = sdr
	}
	return marshalJSON(wire)
}

func mapRateLimitConfig(rl *v1alpha1.RateLimitConfig) json.RawMessage {
	wire := xcRateLimitConfig{
		NoIPAllowedList: rl.NoIPAllowedList,
		IPAllowedList:   mapXCObjectRefs(rl.IPAllowedList),
		NoPolicies:      rl.NoPolicies,
	}
	if rl.RateLimiter != nil {
		wire.RateLimiter = &xcRateLimiterInline{
			TotalNumber:     rl.RateLimiter.TotalNumber,
			Unit:            rl.RateLimiter.Unit,
			BurstMultiplier: rl.RateLimiter.BurstMultiplier,
		}
	}
	if rl.Policies != nil {
		wire.Policies = &xcRateLimitPolicies{
			Policies: mapXCObjectRefs(rl.Policies.Policies),
		}
	}
	return marshalJSON(wire)
}

func mapPolicyBasedChallengeConfig(pbc *v1alpha1.PolicyBasedChallengeConfig) json.RawMessage {
	wire := xcPolicyBasedChallengeConfig{
		DefaultMitigationSettings:     pbc.DefaultMitigationSettings,
		AlwaysEnableJSChallenge:       pbc.AlwaysEnableJSChallenge,
		AlwaysEnableCaptcha:           pbc.AlwaysEnableCaptcha,
		NoChallenge:                   pbc.NoChallenge,
		MaliciousUserMitigationBypass: pbc.MaliciousUserMitigationBypass,
	}
	if pbc.DefaultJSChallengeParameters != nil {
		wire.DefaultJSChallengeParameters = &xcJSChallengeConfig{
			JSScriptDelay: pbc.DefaultJSChallengeParameters.JSScriptDelay,
			CookieExpiry:  pbc.DefaultJSChallengeParameters.CookieExpiry,
			CustomPage:    pbc.DefaultJSChallengeParameters.CustomPage,
		}
	}
	if pbc.DefaultCaptchaChallengeParameters != nil {
		wire.DefaultCaptchaChallengeParameters = &xcCaptchaChallengeConfig{
			Expiry:     pbc.DefaultCaptchaChallengeParameters.Expiry,
			CustomPage: pbc.DefaultCaptchaChallengeParameters.CustomPage,
		}
	}
	if pbc.RuleList != nil {
		rl := &xcChallengeRuleList{}
		for _, r := range pbc.RuleList.Rules {
			xr := xcChallengeRule{Metadata: r.Metadata}
			if r.Spec != nil {
				xr.Spec = &xcChallengeRuleSpec{ChallengeAction: r.Spec.ChallengeAction}
			}
			rl.Rules = append(rl.Rules, xr)
		}
		wire.RuleList = rl
	}
	if pbc.TemporaryBlockingParameters != nil {
		wire.TemporaryBlockingParameters = &xcTemporaryBlockingParams{
			Duration: pbc.TemporaryBlockingParameters.Duration,
		}
	}
	return marshalJSON(wire)
}

func mapRingHash(rh *v1alpha1.RingHashConfig) json.RawMessage {
	wire := xcRingHashConfig{}
	for _, hp := range rh.HashPolicy {
		xhp := xcHashPolicy{
			HeaderName: hp.HeaderName,
			SourceIP:   hp.SourceIP,
			Terminal:   hp.Terminal,
		}
		if hp.Cookie != nil {
			xhp.Cookie = &xcCookieForHashing{
				Name: hp.Cookie.Name,
				TTL:  hp.Cookie.TTL,
				Path: hp.Cookie.Path,
			}
		}
		wire.HashPolicy = append(wire.HashPolicy, xhp)
	}
	return marshalJSON(wire)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildHTTPLoadBalancer -v -count=1`
Expected: all HTTP LB mapper tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/v1alpha1/httplb_types.go internal/controller/httplb_mapper.go internal/controller/httplb_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(httplb): replace opaque JSON fields with typed structs"
```

---

### Task 8: Code Generation and CRD Manifests

**Files:**
- Regenerated: `api/v1alpha1/zz_generated.deepcopy.go`
- Regenerated: `config/crd/bases/*.yaml`
- Regenerated: `config/rbac/*.yaml`
- Updated: `charts/f5xc-k8s-operator/crds/*.yaml`
- Updated: `charts/f5xc-k8s-operator/Chart.yaml`

- [ ] **Step 1: Run full code generation**

Run: `make generate`
Expected: deepcopy regenerated cleanly

- [ ] **Step 2: Run CRD manifest generation and Helm sync**

Run: `make manifests`
Expected: CRD YAML regenerated with typed fields, Helm chart CRDs updated, chart version bumped

- [ ] **Step 3: Verify CRDs have typed schemas (spot check)**

Run: `grep -A 5 "disableOcspStapling:" config/crd/bases/xc.f5.com_certificates.yaml | head -10`
Expected: field shows `type: object` (empty struct) instead of the previous untyped JSON schema

- [ ] **Step 4: Commit**

```bash
git add config/ charts/ api/v1alpha1/zz_generated.deepcopy.go
git commit -m "build: regenerate CRD manifests and Helm chart for typed fields"
```

---

### Task 9: Full Test Suite Validation

**Files:**
- May modify: any controller test files that reference `apiextensionsv1.JSON` for the updated CRD fields

- [ ] **Step 1: Run the full test suite**

Run: `make test`
Expected: all tests pass. If any controller tests fail because they construct CRs with `*apiextensionsv1.JSON` for fields that changed, update those tests to use the new typed structs.

- [ ] **Step 2: Fix any compilation or test failures**

If controller integration tests (`*_controller_test.go`) construct CRs with the old JSON fields, update them:
- `&apiextensionsv1.JSON{Raw: []byte(`{}`)}` → `&struct{}{}`
- `&apiextensionsv1.JSON{Raw: json.RawMessage(`{"field":"value"}`)}` → proper typed struct

Run: `make test`
Expected: all tests PASS

- [ ] **Step 3: Run go vet**

Run: `make vet`
Expected: no issues

- [ ] **Step 4: Commit any test fixes**

```bash
git add internal/controller/
git commit -m "fix: update controller tests for typed CRD fields"
```

---

### Task 10: Contract Test Validation

**Files:**
- No files modified — contract tests use xcclient types directly, which are unchanged

- [ ] **Step 1: Verify contract tests still compile**

Run: `go build -tags contract ./internal/xcclient/`
Expected: compiles successfully (contract tests use `json.RawMessage` directly, not CRD types)

- [ ] **Step 2: Run contract tests against real XC tenant**

Run: `make test-contract`
Expected: all 19 contract tests PASS. The contract tests construct xcclient payloads directly with `json.RawMessage` — they don't use CRD types, so they should be unaffected.

- [ ] **Step 3: Commit and done**

No commit needed if nothing changed. If contract test fixes were needed:

```bash
git add internal/xcclient/
git commit -m "fix: contract test updates for typed fields"
```
