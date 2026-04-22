# New CRDs and HTTP LB References Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three standalone CRDs (APIDefinition, UserIdentification, MaliciousUserMitigation) with full operator stacks, and update the HTTP LB to reference them.

**Architecture:** Each CRD follows the established pattern: types in `api/v1alpha1/`, xcclient CRUD in `internal/xcclient/`, controller+mapper in `internal/controller/`, mapper tests covering all OneOf branches. The HTTP LB gets new ObjectRef and EmptyObject fields plus mapper logic. Wire-format types use snake_case JSON tags and live in the mapper files.

**Tech Stack:** Go, controller-runtime, kubebuilder markers, testify

---

### Task 1: Shared types and xcclient resource constants

**Files:**
- Modify: `api/v1alpha1/shared_types.go`
- Modify: `internal/xcclient/types.go`

- [ ] **Step 1: Add APIOperation to shared_types.go**

Add after the `LabelSelector` struct at the end of the file:

```go
// APIOperation identifies an API endpoint by HTTP method and path.
type APIOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
```

- [ ] **Step 2: Add resource constants to xcclient/types.go**

Add to the const block (after `ResourceCertificate`):

```go
ResourceAPIDefinition           = "api_definitions"
ResourceUserIdentification      = "user_identifications"
ResourceMaliciousUserMitigation = "malicious_user_mitigations"
```

- [ ] **Step 3: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add api/v1alpha1/shared_types.go internal/xcclient/types.go
git commit -m "feat: add APIOperation shared type and 3 new resource constants"
```

---

### Task 2: APIDefinition — types, xcclient, mapper, mapper tests

**Files:**
- Create: `api/v1alpha1/apidefinition_types.go`
- Create: `internal/xcclient/apidefinition.go`
- Create: `internal/controller/apidefinition_mapper.go`
- Create: `internal/controller/apidefinition_mapper_test.go`
- Modify: `internal/xcclient/interface.go`

- [ ] **Step 1: Write the mapper test file**

Create `internal/controller/apidefinition_mapper_test.go`:

```go
package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleAPIDefinition(name, namespace string) *v1alpha1.APIDefinition {
	return &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:  namespace,
			SwaggerSpecs: []string{"/api/object_store/namespaces/ns/stored_objects/swagger/petstore/v1"},
		},
	}
}

func TestBuildAPIDefinitionCreate_BasicFields(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "default")
	result := buildAPIDefinitionCreate(cr, "default")
	assert.Equal(t, "my-apidef", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildAPIDefinitionCreate_SwaggerSpecs(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	result := buildAPIDefinitionCreate(cr, "ns")

	var spec map[string]json.RawMessage
	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &spec))
	assert.Contains(t, string(spec["swagger_specs"]), "petstore")
}

func TestBuildAPIDefinitionCreate_MixedSchemaOrigin(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-mixed", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:       "ns",
			MixedSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"mixed_schema_origin":{}`)
	assert.NotContains(t, string(raw), "strict_schema_origin")
}

func TestBuildAPIDefinitionCreate_StrictSchemaOrigin(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-strict", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:        "ns",
			StrictSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"strict_schema_origin":{}`)
	assert.NotContains(t, string(raw), "mixed_schema_origin")
}

func TestBuildAPIDefinitionCreate_InventoryLists(t *testing.T) {
	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "apidef-inv", Namespace: "ns"},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace: "ns",
			APIInventoryInclusionList: []v1alpha1.APIOperation{
				{Method: "GET", Path: "/api/users"},
			},
			APIInventoryExclusionList: []v1alpha1.APIOperation{
				{Method: "DELETE", Path: "/api/admin"},
			},
			NonAPIEndpoints: []v1alpha1.APIOperation{
				{Method: "GET", Path: "/health"},
			},
		},
	}
	result := buildAPIDefinitionCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"api_inventory_inclusion_list"`)
	assert.Contains(t, string(raw), `"api_inventory_exclusion_list"`)
	assert.Contains(t, string(raw), `"non_api_endpoints"`)
}

func TestBuildAPIDefinitionReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	result := buildAPIDefinitionReplace(cr, "ns", "rv-3")
	assert.Equal(t, "rv-3", result.Metadata.ResourceVersion)
}

func TestBuildAPIDefinitionDesiredSpecJSON(t *testing.T) {
	cr := sampleAPIDefinition("my-apidef", "ns")
	raw, err := buildAPIDefinitionDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasSwagger := spec["swagger_specs"]
	assert.True(t, hasSwagger)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildAPIDefinition -v`
Expected: FAIL — types and functions not defined

- [ ] **Step 3: Create the CRD types file**

Create `api/v1alpha1/apidefinition_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=apidef
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// APIDefinition is the Schema for the apidefinitions API.
type APIDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIDefinitionSpec   `json:"spec,omitempty"`
	Status APIDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// APIDefinitionList contains a list of APIDefinition.
type APIDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIDefinition `json:"items"`
}

type APIDefinitionSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace               string         `json:"xcNamespace"`
	SwaggerSpecs              []string       `json:"swaggerSpecs,omitempty"`
	APIInventoryInclusionList []APIOperation `json:"apiInventoryInclusionList,omitempty"`
	APIInventoryExclusionList []APIOperation `json:"apiInventoryExclusionList,omitempty"`
	NonAPIEndpoints           []APIOperation `json:"nonAPIEndpoints,omitempty"`

	// OneOf: schema_updates_strategy
	MixedSchemaOrigin  *EmptyObject `json:"mixedSchemaOrigin,omitempty"`
	StrictSchemaOrigin *EmptyObject `json:"strictSchemaOrigin,omitempty"`
}

type APIDefinitionStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&APIDefinition{}, &APIDefinitionList{})
}
```

- [ ] **Step 4: Create the xcclient file**

Create `internal/xcclient/apidefinition.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type APIDefinitionSpec struct {
	SwaggerSpecs              json.RawMessage `json:"swagger_specs,omitempty"`
	APIInventoryInclusionList json.RawMessage `json:"api_inventory_inclusion_list,omitempty"`
	APIInventoryExclusionList json.RawMessage `json:"api_inventory_exclusion_list,omitempty"`
	NonAPIEndpoints           json.RawMessage `json:"non_api_endpoints,omitempty"`
	MixedSchemaOrigin         json.RawMessage `json:"mixed_schema_origin,omitempty"`
	StrictSchemaOrigin        json.RawMessage `json:"strict_schema_origin,omitempty"`
}

type APIDefinitionCreate struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     APIDefinitionSpec `json:"spec"`
}

type APIDefinitionReplace struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     APIDefinitionSpec `json:"spec"`
}

type APIDefinition struct {
	Metadata       ObjectMeta        `json:"metadata"`
	SystemMetadata SystemMeta        `json:"system_metadata,omitempty"`
	Spec           APIDefinitionSpec `json:"spec"`
	RawSpec        json.RawMessage   `json:"-"`
}

func (c *Client) CreateAPIDefinition(ctx context.Context, ns string, ad *APIDefinitionCreate) (*APIDefinition, error) {
	ad.Metadata.Namespace = ns
	var result APIDefinition
	if err := c.do(ctx, http.MethodPost, ResourceAPIDefinition, ns, "", ad, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAPIDefinition(ctx context.Context, ns, name string) (*APIDefinition, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceAPIDefinition, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result APIDefinition
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceAPIDefinition(ctx context.Context, ns, name string, ad *APIDefinitionReplace) (*APIDefinition, error) {
	ad.Metadata.Namespace = ns
	ad.Metadata.Name = name
	var result APIDefinition
	if err := c.do(ctx, http.MethodPut, ResourceAPIDefinition, ns, name, ad, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAPIDefinition(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceAPIDefinition, ns, name, nil, nil)
}

func (c *Client) ListAPIDefinitions(ctx context.Context, ns string) ([]*APIDefinition, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceAPIDefinition, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[APIDefinition](raw)
}
```

- [ ] **Step 5: Add APIDefinition methods to the XCClient interface**

In `internal/xcclient/interface.go`, add after the Certificate block:

```go
	// APIDefinition
	CreateAPIDefinition(ctx context.Context, ns string, ad *APIDefinitionCreate) (*APIDefinition, error)
	GetAPIDefinition(ctx context.Context, ns, name string) (*APIDefinition, error)
	ReplaceAPIDefinition(ctx context.Context, ns, name string, ad *APIDefinitionReplace) (*APIDefinition, error)
	DeleteAPIDefinition(ctx context.Context, ns, name string) error
	ListAPIDefinitions(ctx context.Context, ns string) ([]*APIDefinition, error)
```

- [ ] **Step 6: Create the mapper file**

Create `internal/controller/apidefinition_mapper.go`:

```go
package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcAPIOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func buildAPIDefinitionCreate(cr *v1alpha1.APIDefinition, xcNamespace string) *xcclient.APIDefinitionCreate {
	return &xcclient.APIDefinitionCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapAPIDefinitionSpec(&cr.Spec),
	}
}

func buildAPIDefinitionReplace(cr *v1alpha1.APIDefinition, xcNamespace, resourceVersion string) *xcclient.APIDefinitionReplace {
	return &xcclient.APIDefinitionReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapAPIDefinitionSpec(&cr.Spec),
	}
}

func buildAPIDefinitionDesiredSpecJSON(cr *v1alpha1.APIDefinition, xcNamespace string) (json.RawMessage, error) {
	create := buildAPIDefinitionCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapAPIDefinitionSpec(spec *v1alpha1.APIDefinitionSpec) xcclient.APIDefinitionSpec {
	var out xcclient.APIDefinitionSpec

	if len(spec.SwaggerSpecs) > 0 {
		out.SwaggerSpecs = marshalJSON(spec.SwaggerSpecs)
	}

	if len(spec.APIInventoryInclusionList) > 0 {
		out.APIInventoryInclusionList = marshalJSON(mapXCAPIOperations(spec.APIInventoryInclusionList))
	}
	if len(spec.APIInventoryExclusionList) > 0 {
		out.APIInventoryExclusionList = marshalJSON(mapXCAPIOperations(spec.APIInventoryExclusionList))
	}
	if len(spec.NonAPIEndpoints) > 0 {
		out.NonAPIEndpoints = marshalJSON(mapXCAPIOperations(spec.NonAPIEndpoints))
	}

	if spec.MixedSchemaOrigin != nil {
		out.MixedSchemaOrigin = emptyObjectJSON
	}
	if spec.StrictSchemaOrigin != nil {
		out.StrictSchemaOrigin = emptyObjectJSON
	}

	return out
}

func mapXCAPIOperations(ops []v1alpha1.APIOperation) []xcAPIOperation {
	out := make([]xcAPIOperation, len(ops))
	for i, op := range ops {
		out[i] = xcAPIOperation{Method: op.Method, Path: op.Path}
	}
	return out
}
```

- [ ] **Step 7: Run `make generate` to generate deepcopy**

Run: `PATH="/Users/kevin/go/bin:$PATH" make generate`
Expected: `zz_generated.deepcopy.go` updated with APIDefinition methods

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildAPIDefinition -v`
Expected: All 7 tests PASS

- [ ] **Step 9: Commit**

```bash
git add api/v1alpha1/apidefinition_types.go internal/xcclient/apidefinition.go \
  internal/xcclient/interface.go internal/controller/apidefinition_mapper.go \
  internal/controller/apidefinition_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add APIDefinition CRD types, xcclient, and mapper"
```

---

### Task 3: UserIdentification — types, xcclient, mapper, mapper tests

**Files:**
- Create: `api/v1alpha1/useridentification_types.go`
- Create: `internal/xcclient/useridentification.go`
- Create: `internal/controller/useridentification_mapper.go`
- Create: `internal/controller/useridentification_mapper_test.go`
- Modify: `internal/xcclient/interface.go`

- [ ] **Step 1: Write the mapper test file**

Create `internal/controller/useridentification_mapper_test.go`:

```go
package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleUserIdentification(name, namespace string) *v1alpha1.UserIdentification {
	return &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: namespace,
			Rules: []v1alpha1.UserIdentificationRule{
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
}

func TestBuildUserIdentificationCreate_BasicFields(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "default")
	result := buildUserIdentificationCreate(cr, "default")
	assert.Equal(t, "my-uid", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildUserIdentificationCreate_ClientIPRule(t *testing.T) {
	cr := sampleUserIdentification("uid-ip", "ns")
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"client_ip":{}`)
}

func TestBuildUserIdentificationCreate_CookieNameRule(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-cookie", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{CookieName: "Session"},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"cookie_name":"Session"`)
}

func TestBuildUserIdentificationCreate_HTTPHeaderRule(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-header", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{HTTPHeaderName: "Authorization"},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"http_header_name":"Authorization"`)
}

func TestBuildUserIdentificationCreate_MultipleRules(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-multi", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{JWTClaimName: "sub"},
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"jwt_claim_name":"sub"`)
	assert.Contains(t, string(raw), `"client_ip":{}`)
}

func TestBuildUserIdentificationCreate_TLSFingerprint(t *testing.T) {
	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{Name: "uid-tls", Namespace: "ns"},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: "ns",
			Rules: []v1alpha1.UserIdentificationRule{
				{IPAndTLSFingerprint: &v1alpha1.EmptyObject{}},
			},
		},
	}
	result := buildUserIdentificationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"ip_and_tls_fingerprint":{}`)
}

func TestBuildUserIdentificationReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "ns")
	result := buildUserIdentificationReplace(cr, "ns", "rv-7")
	assert.Equal(t, "rv-7", result.Metadata.ResourceVersion)
}

func TestBuildUserIdentificationDesiredSpecJSON(t *testing.T) {
	cr := sampleUserIdentification("my-uid", "ns")
	raw, err := buildUserIdentificationDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasRules := spec["rules"]
	assert.True(t, hasRules)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildUserIdentification -v`
Expected: FAIL

- [ ] **Step 3: Create the CRD types file**

Create `api/v1alpha1/useridentification_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=uid
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// UserIdentification is the Schema for the useridentifications API.
type UserIdentification struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserIdentificationSpec   `json:"spec,omitempty"`
	Status UserIdentificationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserIdentificationList contains a list of UserIdentification.
type UserIdentificationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserIdentification `json:"items"`
}

type UserIdentificationSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace string                   `json:"xcNamespace"`
	Rules       []UserIdentificationRule `json:"rules"`
}

// UserIdentificationRule defines a single identifier rule. Exactly one field must be set.
type UserIdentificationRule struct {
	None                   *EmptyObject `json:"none,omitempty"`
	ClientIP               *EmptyObject `json:"clientIP,omitempty"`
	ClientASN              *EmptyObject `json:"clientASN,omitempty"`
	ClientCity             *EmptyObject `json:"clientCity,omitempty"`
	ClientCountry          *EmptyObject `json:"clientCountry,omitempty"`
	ClientRegion           *EmptyObject `json:"clientRegion,omitempty"`
	CookieName             string       `json:"cookieName,omitempty"`
	HTTPHeaderName         string       `json:"httpHeaderName,omitempty"`
	IPAndHTTPHeaderName    string       `json:"ipAndHTTPHeaderName,omitempty"`
	IPAndTLSFingerprint    *EmptyObject `json:"ipAndTLSFingerprint,omitempty"`
	IPAndJA4TLSFingerprint *EmptyObject `json:"ipAndJA4TLSFingerprint,omitempty"`
	TLSFingerprint         *EmptyObject `json:"tlsFingerprint,omitempty"`
	JA4TLSFingerprint      *EmptyObject `json:"ja4TLSFingerprint,omitempty"`
	JWTClaimName           string       `json:"jwtClaimName,omitempty"`
	QueryParamKey          string       `json:"queryParamKey,omitempty"`
}

type UserIdentificationStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&UserIdentification{}, &UserIdentificationList{})
}
```

- [ ] **Step 4: Create the xcclient file**

Create `internal/xcclient/useridentification.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type UserIdentificationSpec struct {
	Rules json.RawMessage `json:"rules,omitempty"`
}

type UserIdentificationCreate struct {
	Metadata ObjectMeta             `json:"metadata"`
	Spec     UserIdentificationSpec `json:"spec"`
}

type UserIdentificationReplace struct {
	Metadata ObjectMeta             `json:"metadata"`
	Spec     UserIdentificationSpec `json:"spec"`
}

type UserIdentification struct {
	Metadata       ObjectMeta             `json:"metadata"`
	SystemMetadata SystemMeta             `json:"system_metadata,omitempty"`
	Spec           UserIdentificationSpec `json:"spec"`
	RawSpec        json.RawMessage        `json:"-"`
}

func (c *Client) CreateUserIdentification(ctx context.Context, ns string, ui *UserIdentificationCreate) (*UserIdentification, error) {
	ui.Metadata.Namespace = ns
	var result UserIdentification
	if err := c.do(ctx, http.MethodPost, ResourceUserIdentification, ns, "", ui, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetUserIdentification(ctx context.Context, ns, name string) (*UserIdentification, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceUserIdentification, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result UserIdentification
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceUserIdentification(ctx context.Context, ns, name string, ui *UserIdentificationReplace) (*UserIdentification, error) {
	ui.Metadata.Namespace = ns
	ui.Metadata.Name = name
	var result UserIdentification
	if err := c.do(ctx, http.MethodPut, ResourceUserIdentification, ns, name, ui, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteUserIdentification(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceUserIdentification, ns, name, nil, nil)
}

func (c *Client) ListUserIdentifications(ctx context.Context, ns string) ([]*UserIdentification, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceUserIdentification, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[UserIdentification](raw)
}
```

- [ ] **Step 5: Add UserIdentification methods to the XCClient interface**

In `internal/xcclient/interface.go`, add after the APIDefinition block:

```go
	// UserIdentification
	CreateUserIdentification(ctx context.Context, ns string, ui *UserIdentificationCreate) (*UserIdentification, error)
	GetUserIdentification(ctx context.Context, ns, name string) (*UserIdentification, error)
	ReplaceUserIdentification(ctx context.Context, ns, name string, ui *UserIdentificationReplace) (*UserIdentification, error)
	DeleteUserIdentification(ctx context.Context, ns, name string) error
	ListUserIdentifications(ctx context.Context, ns string) ([]*UserIdentification, error)
```

- [ ] **Step 6: Create the mapper file**

Create `internal/controller/useridentification_mapper.go`:

```go
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
```

- [ ] **Step 7: Run `make generate`**

Run: `PATH="/Users/kevin/go/bin:$PATH" make generate`

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildUserIdentification -v`
Expected: All 8 tests PASS

- [ ] **Step 9: Commit**

```bash
git add api/v1alpha1/useridentification_types.go internal/xcclient/useridentification.go \
  internal/xcclient/interface.go internal/controller/useridentification_mapper.go \
  internal/controller/useridentification_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add UserIdentification CRD types, xcclient, and mapper"
```

---

### Task 4: MaliciousUserMitigation — types, xcclient, mapper, mapper tests

**Files:**
- Create: `api/v1alpha1/malicioususermitigation_types.go`
- Create: `internal/xcclient/malicioususermitigation.go`
- Create: `internal/controller/malicioususermitigation_mapper.go`
- Create: `internal/controller/malicioususermitigation_mapper_test.go`
- Modify: `internal/xcclient/interface.go`

- [ ] **Step 1: Write the mapper test file**

Create `internal/controller/malicioususermitigation_mapper_test.go`:

```go
package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleMaliciousUserMitigation(name, namespace string) *v1alpha1.MaliciousUserMitigation {
	return &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: namespace,
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
}

func TestBuildMaliciousUserMitigationCreate_BasicFields(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "default")
	result := buildMaliciousUserMitigationCreate(cr, "default")
	assert.Equal(t, "my-mum", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
}

func TestBuildMaliciousUserMitigationCreate_AllThreatLevels(t *testing.T) {
	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: "mum-all", Namespace: "ns"},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: "ns",
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Medium: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{CaptchaChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{High: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{BlockTemporarily: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
	result := buildMaliciousUserMitigationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	s := string(raw)
	assert.Contains(t, s, `"low":{}`)
	assert.Contains(t, s, `"medium":{}`)
	assert.Contains(t, s, `"high":{}`)
	assert.Contains(t, s, `"javascript_challenge":{}`)
	assert.Contains(t, s, `"captcha_challenge":{}`)
	assert.Contains(t, s, `"block_temporarily":{}`)
}

func TestBuildMaliciousUserMitigationCreate_NilMitigationType(t *testing.T) {
	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{Name: "mum-nil", Namespace: "ns"},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: "ns",
		},
	}
	result := buildMaliciousUserMitigationCreate(cr, "ns")

	raw, err := json.Marshal(result.Spec)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "mitigation_type")
}

func TestBuildMaliciousUserMitigationReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "ns")
	result := buildMaliciousUserMitigationReplace(cr, "ns", "rv-2")
	assert.Equal(t, "rv-2", result.Metadata.ResourceVersion)
}

func TestBuildMaliciousUserMitigationDesiredSpecJSON(t *testing.T) {
	cr := sampleMaliciousUserMitigation("my-mum", "ns")
	raw, err := buildMaliciousUserMitigationDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasMT := spec["mitigation_type"]
	assert.True(t, hasMT)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/controller/ -run TestBuildMaliciousUserMitigation -v`
Expected: FAIL

- [ ] **Step 3: Create the CRD types file**

Create `api/v1alpha1/malicioususermitigation_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mum
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MaliciousUserMitigation is the Schema for the malicioususermitigations API.
type MaliciousUserMitigation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MaliciousUserMitigationSpec   `json:"spec,omitempty"`
	Status MaliciousUserMitigationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MaliciousUserMitigationList contains a list of MaliciousUserMitigation.
type MaliciousUserMitigationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MaliciousUserMitigation `json:"items"`
}

type MaliciousUserMitigationSpec struct {
	// +kubebuilder:validation:Required
	XCNamespace    string                       `json:"xcNamespace"`
	MitigationType *MaliciousUserMitigationType `json:"mitigationType,omitempty"`
}

type MaliciousUserMitigationType struct {
	Rules []MaliciousUserMitigationRule `json:"rules"`
}

type MaliciousUserMitigationRule struct {
	ThreatLevel      MaliciousUserThreatLevel      `json:"threatLevel"`
	MitigationAction MaliciousUserMitigationAction `json:"mitigationAction"`
}

type MaliciousUserThreatLevel struct {
	Low    *EmptyObject `json:"low,omitempty"`
	Medium *EmptyObject `json:"medium,omitempty"`
	High   *EmptyObject `json:"high,omitempty"`
}

type MaliciousUserMitigationAction struct {
	BlockTemporarily    *EmptyObject `json:"blockTemporarily,omitempty"`
	CaptchaChallenge    *EmptyObject `json:"captchaChallenge,omitempty"`
	JavascriptChallenge *EmptyObject `json:"javascriptChallenge,omitempty"`
}

type MaliciousUserMitigationStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MaliciousUserMitigation{}, &MaliciousUserMitigationList{})
}
```

- [ ] **Step 4: Create the xcclient file**

Create `internal/xcclient/malicioususermitigation.go`:

```go
package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
)

type MaliciousUserMitigationSpec struct {
	MitigationType json.RawMessage `json:"mitigation_type,omitempty"`
}

type MaliciousUserMitigationCreate struct {
	Metadata ObjectMeta                  `json:"metadata"`
	Spec     MaliciousUserMitigationSpec `json:"spec"`
}

type MaliciousUserMitigationReplace struct {
	Metadata ObjectMeta                  `json:"metadata"`
	Spec     MaliciousUserMitigationSpec `json:"spec"`
}

type MaliciousUserMitigation struct {
	Metadata       ObjectMeta                  `json:"metadata"`
	SystemMetadata SystemMeta                  `json:"system_metadata,omitempty"`
	Spec           MaliciousUserMitigationSpec `json:"spec"`
	RawSpec        json.RawMessage             `json:"-"`
}

func (c *Client) CreateMaliciousUserMitigation(ctx context.Context, ns string, m *MaliciousUserMitigationCreate) (*MaliciousUserMitigation, error) {
	m.Metadata.Namespace = ns
	var result MaliciousUserMitigation
	if err := c.do(ctx, http.MethodPost, ResourceMaliciousUserMitigation, ns, "", m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetMaliciousUserMitigation(ctx context.Context, ns, name string) (*MaliciousUserMitigation, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceMaliciousUserMitigation, ns, name, nil, &raw); err != nil {
		return nil, err
	}
	var result MaliciousUserMitigation
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.RawSpec = extractRawSpec(raw)
	return &result, nil
}

func (c *Client) ReplaceMaliciousUserMitigation(ctx context.Context, ns, name string, m *MaliciousUserMitigationReplace) (*MaliciousUserMitigation, error) {
	m.Metadata.Namespace = ns
	m.Metadata.Name = name
	var result MaliciousUserMitigation
	if err := c.do(ctx, http.MethodPut, ResourceMaliciousUserMitigation, ns, name, m, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteMaliciousUserMitigation(ctx context.Context, ns, name string) error {
	return c.do(ctx, http.MethodDelete, ResourceMaliciousUserMitigation, ns, name, nil, nil)
}

func (c *Client) ListMaliciousUserMitigations(ctx context.Context, ns string) ([]*MaliciousUserMitigation, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, ResourceMaliciousUserMitigation, ns, "", nil, &raw); err != nil {
		return nil, err
	}
	return unmarshalList[MaliciousUserMitigation](raw)
}
```

- [ ] **Step 5: Add MaliciousUserMitigation methods to the XCClient interface**

In `internal/xcclient/interface.go`, add after the UserIdentification block:

```go
	// MaliciousUserMitigation
	CreateMaliciousUserMitigation(ctx context.Context, ns string, m *MaliciousUserMitigationCreate) (*MaliciousUserMitigation, error)
	GetMaliciousUserMitigation(ctx context.Context, ns, name string) (*MaliciousUserMitigation, error)
	ReplaceMaliciousUserMitigation(ctx context.Context, ns, name string, m *MaliciousUserMitigationReplace) (*MaliciousUserMitigation, error)
	DeleteMaliciousUserMitigation(ctx context.Context, ns, name string) error
	ListMaliciousUserMitigations(ctx context.Context, ns string) ([]*MaliciousUserMitigation, error)
```

- [ ] **Step 6: Create the mapper file**

Create `internal/controller/malicioususermitigation_mapper.go`:

```go
package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcMaliciousUserMitigationType struct {
	Rules []xcMaliciousUserMitigationRule `json:"rules"`
}

type xcMaliciousUserMitigationRule struct {
	ThreatLevel      xcMaliciousUserThreatLevel      `json:"threat_level"`
	MitigationAction xcMaliciousUserMitigationAction `json:"mitigation_action"`
}

type xcMaliciousUserThreatLevel struct {
	Low    *v1alpha1.EmptyObject `json:"low,omitempty"`
	Medium *v1alpha1.EmptyObject `json:"medium,omitempty"`
	High   *v1alpha1.EmptyObject `json:"high,omitempty"`
}

type xcMaliciousUserMitigationAction struct {
	BlockTemporarily    *v1alpha1.EmptyObject `json:"block_temporarily,omitempty"`
	CaptchaChallenge    *v1alpha1.EmptyObject `json:"captcha_challenge,omitempty"`
	JavascriptChallenge *v1alpha1.EmptyObject `json:"javascript_challenge,omitempty"`
}

func buildMaliciousUserMitigationCreate(cr *v1alpha1.MaliciousUserMitigation, xcNamespace string) *xcclient.MaliciousUserMitigationCreate {
	return &xcclient.MaliciousUserMitigationCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapMaliciousUserMitigationSpec(&cr.Spec),
	}
}

func buildMaliciousUserMitigationReplace(cr *v1alpha1.MaliciousUserMitigation, xcNamespace, resourceVersion string) *xcclient.MaliciousUserMitigationReplace {
	return &xcclient.MaliciousUserMitigationReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapMaliciousUserMitigationSpec(&cr.Spec),
	}
}

func buildMaliciousUserMitigationDesiredSpecJSON(cr *v1alpha1.MaliciousUserMitigation, xcNamespace string) (json.RawMessage, error) {
	create := buildMaliciousUserMitigationCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapMaliciousUserMitigationSpec(spec *v1alpha1.MaliciousUserMitigationSpec) xcclient.MaliciousUserMitigationSpec {
	var out xcclient.MaliciousUserMitigationSpec
	if spec.MitigationType != nil {
		var rules []xcMaliciousUserMitigationRule
		for _, r := range spec.MitigationType.Rules {
			rules = append(rules, xcMaliciousUserMitigationRule{
				ThreatLevel: xcMaliciousUserThreatLevel{
					Low: r.ThreatLevel.Low, Medium: r.ThreatLevel.Medium, High: r.ThreatLevel.High,
				},
				MitigationAction: xcMaliciousUserMitigationAction{
					BlockTemporarily: r.MitigationAction.BlockTemporarily, CaptchaChallenge: r.MitigationAction.CaptchaChallenge, JavascriptChallenge: r.MitigationAction.JavascriptChallenge,
				},
			})
		}
		out.MitigationType = marshalJSON(xcMaliciousUserMitigationType{Rules: rules})
	}
	return out
}
```

- [ ] **Step 7: Run `make generate`**

Run: `PATH="/Users/kevin/go/bin:$PATH" make generate`

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestBuildMaliciousUserMitigation -v`
Expected: All 5 tests PASS

- [ ] **Step 9: Commit**

```bash
git add api/v1alpha1/malicioususermitigation_types.go internal/xcclient/malicioususermitigation.go \
  internal/xcclient/interface.go internal/controller/malicioususermitigation_mapper.go \
  internal/controller/malicioususermitigation_mapper_test.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add MaliciousUserMitigation CRD types, xcclient, and mapper"
```

---

### Task 5: Controllers for all 3 new CRDs

**Files:**
- Create: `internal/controller/apidefinition_controller.go`
- Create: `internal/controller/useridentification_controller.go`
- Create: `internal/controller/malicioususermitigation_controller.go`

All three follow the exact AppFirewall controller pattern (no external secret reading). Each controller has: Reconcile, handleCreate, handleUpdate, handleDeletion, handleXCError, setStatus, setCondition, SetupWithManager.

- [ ] **Step 1: Create `internal/controller/apidefinition_controller.go`**

Follow the exact same pattern as `appfirewall_controller.go`. Replace all occurrences:
- `AppFirewall` → `APIDefinition`
- `appfirewall` → `apidefinition`
- `app firewall` → `API definition`
- `app_firewalls` → `api_definitions`
- `AppFirewallReconciler` → `APIDefinitionReconciler`
- `buildAppFirewallCreate` → `buildAPIDefinitionCreate`
- `buildAppFirewallReplace` → `buildAPIDefinitionReplace`
- `buildAppFirewallDesiredSpecJSON` → `buildAPIDefinitionDesiredSpecJSON`
- `xc.CreateAppFirewall` → `xc.CreateAPIDefinition`
- `xc.GetAppFirewall` → `xc.GetAPIDefinition`
- `xc.ReplaceAppFirewall` → `xc.ReplaceAPIDefinition`
- `xc.DeleteAppFirewall` → `xc.DeleteAPIDefinition`
- `*xcclient.AppFirewall` → `*xcclient.APIDefinition`
- RBAC markers: `appfirewalls` → `apidefinitions`

- [ ] **Step 2: Create `internal/controller/useridentification_controller.go`**

Same pattern. Replace:
- `AppFirewall` → `UserIdentification`
- `appfirewall` → `useridentification`
- `app firewall` → `user identification`
- RBAC markers: `appfirewalls` → `useridentifications`

- [ ] **Step 3: Create `internal/controller/malicioususermitigation_controller.go`**

Same pattern. Replace:
- `AppFirewall` → `MaliciousUserMitigation`
- `appfirewall` → `malicioususermitigation`
- `app firewall` → `malicious user mitigation`
- RBAC markers: `appfirewalls` → `malicioususermitigations`

- [ ] **Step 4: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/controller/apidefinition_controller.go \
  internal/controller/useridentification_controller.go \
  internal/controller/malicioususermitigation_controller.go
git commit -m "feat: add controllers for APIDefinition, UserIdentification, MaliciousUserMitigation"
```

---

### Task 6: Register controllers in main.go

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Add 3 new reconciler registrations**

Add after the Certificate reconciler block (line ~187), before `mgr.AddHealthzCheck`:

```go
	if err := (&controller.APIDefinitionReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("APIDefinition"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "APIDefinition")
		os.Exit(1)
	}

	if err := (&controller.UserIdentificationReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("UserIdentification"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "UserIdentification")
		os.Exit(1)
	}

	if err := (&controller.MaliciousUserMitigationReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("MaliciousUserMitigation"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "MaliciousUserMitigation")
		os.Exit(1)
	}
```

- [ ] **Step 2: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "feat: register APIDefinition, UserIdentification, MaliciousUserMitigation controllers"
```

---

### Task 7: HTTP LB types and xcclient updates

**Files:**
- Modify: `api/v1alpha1/httplb_types.go`
- Modify: `internal/xcclient/httplb.go`

- [ ] **Step 1: Add new types and fields to `httplb_types.go`**

Add the `APISpecificationConfig` and `ValidationCustomList` types after the existing `ActiveServicePoliciesConfig` type:

```go
// APISpecificationConfig wraps an APIDefinition reference with validation settings.
type APISpecificationConfig struct {
	APIDefinition              *ObjectRef            `json:"apiDefinition"`
	ValidationDisabled         *EmptyObject          `json:"validationDisabled,omitempty"`
	ValidationAllSpecEndpoints *EmptyObject          `json:"validationAllSpecEndpoints,omitempty"`
	ValidationCustomList       *ValidationCustomList `json:"validationCustomList,omitempty"`
}

// ValidationCustomList holds a custom list of endpoint validations.
type ValidationCustomList struct {
	EndpointValidationList []APIOperation `json:"endpointValidationList,omitempty"`
}
```

Add new fields to `HTTPLoadBalancerSpec` (at the end, before the closing brace):

```go
	// API definition OneOf
	DisableAPIDefinition *EmptyObject           `json:"disableAPIDefinition,omitempty"`
	APISpecification     *APISpecificationConfig `json:"apiSpecification,omitempty"`

	// Malicious user detection OneOf
	DisableMaliciousUserDetection *EmptyObject `json:"disableMaliciousUserDetection,omitempty"`
	EnableMaliciousUserDetection  *EmptyObject `json:"enableMaliciousUserDetection,omitempty"`
```

Expand the User ID OneOf — add after the existing `UserIDClientIP` field:

```go
	UserIdentification *ObjectRef `json:"userIdentification,omitempty"`
```

Add `MaliciousUserMitigation` to the existing `PolicyBasedChallengeConfig` struct:

```go
	MaliciousUserMitigation *ObjectRef `json:"maliciousUserMitigation,omitempty"`
```

- [ ] **Step 2: Add matching fields to `xcclient/httplb.go` HTTPLoadBalancerSpec**

Add to the xcclient `HTTPLoadBalancerSpec` struct:

```go
	// User ID — add after UserIDClientIP
	UserIdentification *ObjectRef `json:"user_identification,omitempty"`

	// API definition — OneOf: disable_api_definition, api_specification
	DisableAPIDefinition json.RawMessage `json:"disable_api_definition,omitempty"`
	APISpecification     json.RawMessage `json:"api_specification,omitempty"`

	// Malicious user detection — OneOf: disable/enable
	DisableMaliciousUserDetection json.RawMessage `json:"disable_malicious_user_detection,omitempty"`
	EnableMaliciousUserDetection  json.RawMessage `json:"enable_malicious_user_detection,omitempty"`
```

- [ ] **Step 3: Run `make generate`**

Run: `PATH="/Users/kevin/go/bin:$PATH" make generate`

- [ ] **Step 4: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/httplb_types.go internal/xcclient/httplb.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add API definition, user identification, and malicious user fields to HTTP LB"
```

---

### Task 8: HTTP LB mapper updates

**Files:**
- Modify: `internal/controller/httplb_mapper.go`
- Modify: `internal/controller/httplb_mapper_test.go`

- [ ] **Step 1: Write failing tests for the new HTTP LB mapper fields**

Add to `internal/controller/httplb_mapper_test.go`:

```go
func TestMapHTTPLoadBalancerSpec_UserIdentification(t *testing.T) {
	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "hlb-uid", Namespace: "ns"},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace:        "ns",
			Domains:            []string{"test.com"},
			UserIdentification: &v1alpha1.ObjectRef{Name: "my-uid-policy", Namespace: "ns"},
		},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	require.NotNil(t, result.Spec.UserIdentification)
	assert.Equal(t, "my-uid-policy", result.Spec.UserIdentification.Name)
}

func TestMapHTTPLoadBalancerSpec_DisableAPIDefinition(t *testing.T) {
	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "hlb-no-api", Namespace: "ns"},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace:          "ns",
			Domains:              []string{"test.com"},
			DisableAPIDefinition: &v1alpha1.EmptyObject{},
		},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.DisableAPIDefinition))
	assert.Nil(t, result.Spec.APISpecification)
}

func TestMapHTTPLoadBalancerSpec_APISpecification(t *testing.T) {
	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "hlb-apispec", Namespace: "ns"},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: "ns",
			Domains:     []string{"test.com"},
			APISpecification: &v1alpha1.APISpecificationConfig{
				APIDefinition:      &v1alpha1.ObjectRef{Name: "my-apidef", Namespace: "ns"},
				ValidationDisabled: &v1alpha1.EmptyObject{},
			},
		},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.APISpecification)
	assert.Contains(t, string(result.Spec.APISpecification), "my-apidef")
	assert.Contains(t, string(result.Spec.APISpecification), "validation_disabled")
}

func TestMapHTTPLoadBalancerSpec_MaliciousUserDetection(t *testing.T) {
	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "hlb-mud", Namespace: "ns"},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace:                  "ns",
			Domains:                      []string{"test.com"},
			EnableMaliciousUserDetection: &v1alpha1.EmptyObject{},
		},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.EnableMaliciousUserDetection))
	assert.Nil(t, result.Spec.DisableMaliciousUserDetection)
}

func TestMapHTTPLoadBalancerSpec_PolicyBasedChallengeWithMitigation(t *testing.T) {
	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: "hlb-pbc-mum", Namespace: "ns"},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: "ns",
			Domains:     []string{"test.com"},
			PolicyBasedChallenge: &v1alpha1.PolicyBasedChallengeConfig{
				MaliciousUserMitigation: &v1alpha1.ObjectRef{Name: "my-mum", Namespace: "ns"},
			},
		},
	}
	result := buildHTTPLoadBalancerCreate(cr, "ns")
	assert.NotNil(t, result.Spec.PolicyBasedChallenge)
	assert.Contains(t, string(result.Spec.PolicyBasedChallenge), "malicious_user_mitigation")
	assert.Contains(t, string(result.Spec.PolicyBasedChallenge), "my-mum")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/controller/ -run TestMapHTTPLoadBalancerSpec_ -v`
Expected: FAIL — new mapper logic not yet implemented

- [ ] **Step 3: Add wire-format types to `httplb_mapper.go`**

Add after the existing `xcActiveServicePoliciesConfig` type:

```go
type xcAPISpecificationConfig struct {
	APIDefinition              *xcObjectRef            `json:"api_definition"`
	ValidationDisabled         *v1alpha1.EmptyObject   `json:"validation_disabled,omitempty"`
	ValidationAllSpecEndpoints *v1alpha1.EmptyObject   `json:"validation_all_spec_endpoints,omitempty"`
	ValidationCustomList       *xcValidationCustomList `json:"validation_custom_list,omitempty"`
}

type xcValidationCustomList struct {
	EndpointValidationList []xcAPIOperation `json:"endpoint_validation_list,omitempty"`
}
```

- [ ] **Step 4: Add mapper logic to `mapHTTPLoadBalancerSpec`**

Add to `mapHTTPLoadBalancerSpec` function, before the `return out`:

```go
	// User ID OneOf (expanded)
	if spec.UserIdentification != nil {
		out.UserIdentification = mapObjectRefPtr(spec.UserIdentification)
	}

	// API definition OneOf
	if spec.DisableAPIDefinition != nil {
		out.DisableAPIDefinition = emptyObjectJSON
	}
	if spec.APISpecification != nil {
		out.APISpecification = mapAPISpecificationConfig(spec.APISpecification)
	}

	// Malicious user detection OneOf
	if spec.DisableMaliciousUserDetection != nil {
		out.DisableMaliciousUserDetection = emptyObjectJSON
	}
	if spec.EnableMaliciousUserDetection != nil {
		out.EnableMaliciousUserDetection = emptyObjectJSON
	}
```

Also update the `mapPolicyBasedChallengeConfig` function to add:

```go
	if pbc.MaliciousUserMitigation != nil {
		wire.MaliciousUserMitigation = mapXCObjectRef(pbc.MaliciousUserMitigation)
	}
```

And update the `xcPolicyBasedChallengeConfig` wire type to add:

```go
	MaliciousUserMitigation *xcObjectRef `json:"malicious_user_mitigation,omitempty"`
```

Add the `mapAPISpecificationConfig` helper:

```go
func mapAPISpecificationConfig(as *v1alpha1.APISpecificationConfig) json.RawMessage {
	wire := xcAPISpecificationConfig{
		APIDefinition:              mapXCObjectRef(as.APIDefinition),
		ValidationDisabled:         as.ValidationDisabled,
		ValidationAllSpecEndpoints: as.ValidationAllSpecEndpoints,
	}
	if as.ValidationCustomList != nil {
		wire.ValidationCustomList = &xcValidationCustomList{
			EndpointValidationList: mapXCAPIOperations(as.ValidationCustomList.EndpointValidationList),
		}
	}
	return marshalJSON(wire)
}
```

Note: `mapXCAPIOperations` was defined in the APIDefinition mapper (Task 2). If the build complains about duplicate definitions, move it to `mapper_helpers.go` instead. Since `xcAPIOperation` is defined in `apidefinition_mapper.go`, the function is already accessible within the `controller` package.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/controller/ -run TestMapHTTPLoadBalancerSpec_ -v`
Expected: All 5 new tests PASS

- [ ] **Step 6: Run full mapper test suite**

Run: `go test ./internal/controller/ -run TestBuild -v`
Expected: All existing + new tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/controller/httplb_mapper.go internal/controller/httplb_mapper_test.go
git commit -m "feat: add HTTP LB mapper for API definition, user identification, malicious user fields"
```

---

### Task 9: Code generation and CRD manifests

**Files:**
- Modified by codegen: `api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `config/rbac/role.yaml`

- [ ] **Step 1: Run make generate**

Run: `PATH="/Users/kevin/go/bin:$PATH" make generate`
Expected: Success

- [ ] **Step 2: Run make manifests**

Run: `PATH="/Users/kevin/go/bin:$PATH" make manifests`
Expected: New CRD manifests generated for APIDefinition, UserIdentification, MaliciousUserMitigation

- [ ] **Step 3: Verify new CRD manifests exist**

Run: `ls config/crd/bases/xc.f5.com_apidefinitions.yaml config/crd/bases/xc.f5.com_useridentifications.yaml config/crd/bases/xc.f5.com_malicioususermitigations.yaml`
Expected: All 3 files exist

- [ ] **Step 4: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/zz_generated.deepcopy.go config/crd/ config/rbac/
git commit -m "chore: regenerate CRD manifests and deepcopy for new resources"
```

---

### Task 10: Full test suite validation

**Files:**
- No new files — validation only

- [ ] **Step 1: Run all mapper unit tests**

Run: `go test ./internal/controller/ -run TestBuild -v -count=1`
Expected: All tests PASS (existing + new)

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Verify full build**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit any fixes**

If any test failures or vet issues found, fix and commit.

---

### Task 11: Contract test stubs for new CRDs

**Files:**
- Modify: `internal/controller/contract_test.go`
- Create: `internal/controller/apidefinition_integration_test.go`
- Create: `internal/controller/useridentification_integration_test.go`
- Create: `internal/controller/malicioususermitigation_integration_test.go`

- [ ] **Step 1: Create integration test waiters**

Create `internal/controller/apidefinition_integration_test.go`:

```go
package controller

import (
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func waitForAPIDefinitionConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.APIDefinition {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.APIDefinition
		if err := testClient.Get(testCtx, key, &cr); err == nil {
			if c := meta.FindStatusCondition(cr.Status.Conditions, condType); c != nil && c.Status == wantStatus {
				return &cr
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s=%s on %s", condType, wantStatus, key)
	return nil
}
```

Create `internal/controller/useridentification_integration_test.go`:

```go
package controller

import (
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func waitForUserIdentificationConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.UserIdentification {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.UserIdentification
		if err := testClient.Get(testCtx, key, &cr); err == nil {
			if c := meta.FindStatusCondition(cr.Status.Conditions, condType); c != nil && c.Status == wantStatus {
				return &cr
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s=%s on %s", condType, wantStatus, key)
	return nil
}
```

Create `internal/controller/malicioususermitigation_integration_test.go`:

```go
package controller

import (
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func waitForMaliciousUserMitigationConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.MaliciousUserMitigation {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.MaliciousUserMitigation
		if err := testClient.Get(testCtx, key, &cr); err == nil {
			if c := meta.FindStatusCondition(cr.Status.Conditions, condType); c != nil && c.Status == wantStatus {
				return &cr
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s=%s on %s", condType, wantStatus, key)
	return nil
}
```

- [ ] **Step 2: Add contract tests to `contract_test.go`**

Add these tests at the end of `contract_test.go`:

```go
// ---------------------------------------------------------------------------
// APIDefinition — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_APIDefinitionCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &APIDefinitionReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-apidef"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteAPIDefinition(context.Background(), xcNS, "contract-apidef")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-apidef",
			Namespace: "contract-apidef",
		},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:       xcNS,
			MixedSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForAPIDefinitionConditionResult(t, types.NamespacedName{Name: "contract-apidef", Namespace: "contract-apidef"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// UserIdentification — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_UserIdentificationCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &UserIdentificationReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-uid"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteUserIdentification(context.Background(), xcNS, "contract-uid")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-uid",
			Namespace: "contract-uid",
		},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: xcNS,
			Rules: []v1alpha1.UserIdentificationRule{
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForUserIdentificationConditionResult(t, types.NamespacedName{Name: "contract-uid", Namespace: "contract-uid"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// MaliciousUserMitigation — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_MaliciousUserMitigationCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &MaliciousUserMitigationReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-mum"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-mum",
			Namespace: "contract-mum",
		},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: xcNS,
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Medium: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{CaptchaChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{High: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{BlockTemporarily: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForMaliciousUserMitigationConditionResult(t, types.NamespacedName{Name: "contract-mum", Namespace: "contract-mum"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}
```

- [ ] **Step 3: Verify contract tests compile**

Run: `go test -tags contract ./internal/controller/ -run TestContract_APIDefinition -v -list '.*'`
Expected: Tests listed (not run — no XC credentials)

- [ ] **Step 4: Commit**

```bash
git add internal/controller/apidefinition_integration_test.go \
  internal/controller/useridentification_integration_test.go \
  internal/controller/malicioususermitigation_integration_test.go \
  internal/controller/contract_test.go
git commit -m "test: add contract tests for APIDefinition, UserIdentification, MaliciousUserMitigation"
```
