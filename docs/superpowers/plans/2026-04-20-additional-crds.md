# Additional CRDs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CRDs and controllers for the remaining 6 F5 XC resource types (HTTPLoadBalancer, TCPLoadBalancer, AppFirewall, HealthCheck, ServicePolicy, RateLimiter), each following the OriginPool reconciliation pattern.

**Architecture:** Each resource gets its own types file, mapper, controller, and full test suite (mapper unit tests, controller unit tests with envtest + fake XC client, integration tests with envtest + FakeXCServer, and contract tests). Shared constants and types are extracted to avoid duplication. All controllers share the same ClientSet and registration pattern in main.go.

**Tech Stack:** Go, controller-runtime v0.23.3, envtest, controller-gen v0.20.1, Kubebuilder markers

---

## Task 1: Extract Shared Constants, Types, and Generalize Test Helpers

**Files:**
- Create: `api/v1alpha1/constants.go`
- Create: `api/v1alpha1/shared_types.go`
- Modify: `api/v1alpha1/originpool_types.go` — remove constants and ObjectRef
- Modify: `internal/controller/originpool_mapper.go` — rename `buildDesiredSpecJSON` → `buildOriginPoolDesiredSpecJSON`
- Modify: `internal/controller/originpool_mapper_test.go` — update renamed function call
- Modify: `internal/controller/originpool_controller.go` — update renamed function call
- Modify: `internal/controller/suite_test.go` — add generic `startManagerFor` helper

- [ ] **Step 1: Create `api/v1alpha1/constants.go`**

```go
package v1alpha1

const (
	FinalizerXCCleanup = "xc.f5.com/cleanup"

	AnnotationXCNamespace    = "f5xc.io/namespace"
	AnnotationDeletionPolicy = "f5xc.io/deletion-policy"

	DeletionPolicyOrphan = "orphan"

	ConditionReady  = "Ready"
	ConditionSynced = "Synced"

	ReasonCreateSucceeded = "CreateSucceeded"
	ReasonUpdateSucceeded = "UpdateSucceeded"
	ReasonUpToDate        = "UpToDate"
	ReasonDeleteSucceeded = "DeleteSucceeded"
	ReasonCreateFailed    = "CreateFailed"
	ReasonUpdateFailed    = "UpdateFailed"
	ReasonDeleteFailed    = "DeleteFailed"
	ReasonAuthFailure     = "AuthFailure"
	ReasonRateLimited     = "RateLimited"
	ReasonServerError     = "ServerError"
	ReasonConflict        = "Conflict"
)
```

- [ ] **Step 2: Create `api/v1alpha1/shared_types.go`**

```go
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
```

- [ ] **Step 3: Remove constants and ObjectRef from `originpool_types.go`**

Remove the entire `const (...)` block (lines 8–30) and the `ObjectRef` type definition (lines 112–116) from `api/v1alpha1/originpool_types.go`. These now live in `constants.go` and `shared_types.go`. The `OriginPool` types still reference `ObjectRef` — it's in the same package so no import changes needed.

- [ ] **Step 4: Rename `buildDesiredSpecJSON` to `buildOriginPoolDesiredSpecJSON`**

In `internal/controller/originpool_mapper.go`, rename the function:

```go
func buildOriginPoolDesiredSpecJSON(cr *v1alpha1.OriginPool, xcNamespace string) (json.RawMessage, error) {
	create := buildOriginPoolCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}
```

In `internal/controller/originpool_controller.go`, update the call site (line 87):

```go
desiredJSON, err := buildOriginPoolDesiredSpecJSON(cr, xcNS)
```

In `internal/controller/originpool_mapper_test.go`, update the test (line 148):

```go
raw, err := buildOriginPoolDesiredSpecJSON(cr, "ns")
```

- [ ] **Step 5: Add `startManagerFor` to `suite_test.go`**

Add this function after the existing `startManager`:

```go
func startManagerFor(t *testing.T, setup func(mgr ctrl.Manager) error) {
	t.Helper()
	mgr, err := ctrl.NewManager(testCfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Controller: config.Controller{
			SkipNameValidation: boolPtr(true),
		},
	})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}
	if err := setup(mgr); err != nil {
		t.Fatalf("setting up controller: %v", err)
	}
	go func() {
		if err := mgr.Start(testCtx); err != nil {
			t.Errorf("manager exited with error: %v", err)
		}
	}()
}
```

- [ ] **Step 6: Run tests to verify nothing broke**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1`
Expected: All existing tests pass

- [ ] **Step 7: Run controller-gen to regenerate deepcopy**

Run: `controller-gen object paths="./api/..."`
Then: `controller-gen crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac`

Verify that `zz_generated.deepcopy.go` now includes `DeepCopyInto` for `RoutePool`.

- [ ] **Step 8: Commit**

```bash
git add api/v1alpha1/constants.go api/v1alpha1/shared_types.go api/v1alpha1/originpool_types.go api/v1alpha1/zz_generated.deepcopy.go internal/controller/originpool_mapper.go internal/controller/originpool_mapper_test.go internal/controller/originpool_controller.go internal/controller/suite_test.go config/crd/ config/rbac/
git commit -m "Extract shared constants and types, generalize test helpers"
```

---

## Task 2: RateLimiter CRD — Types, Mapper, Controller, Tests

The simplest resource: 3 typed fields (Threshold, Unit, BurstMultiplier). XC client uses value types.

**Files:**
- Create: `api/v1alpha1/ratelimiter_types.go`
- Create: `internal/controller/ratelimiter_mapper.go`
- Create: `internal/controller/ratelimiter_mapper_test.go`
- Create: `internal/controller/ratelimiter_controller.go`
- Create: `internal/controller/ratelimiter_controller_test.go`
- Create: `internal/controller/ratelimiter_integration_test.go`

- [ ] **Step 1: Write mapper test file `internal/controller/ratelimiter_mapper_test.go`**

```go
package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func uint32Ptr(v uint32) *uint32 { return &v }

func TestBuildRateLimiterCreate_BasicFields(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "default"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterCreate(cr, "default")
	assert.Equal(t, "my-rl", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.Equal(t, uint32(100), result.Spec.Threshold)
	assert.Equal(t, "MINUTE", result.Spec.Unit)
	assert.Equal(t, uint32(0), result.Spec.BurstMultiplier)
}

func TestBuildRateLimiterCreate_WithBurstMultiplier(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "burst-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold:       50,
			Unit:            "SECOND",
			BurstMultiplier: uint32Ptr(3),
		},
	}

	result := buildRateLimiterCreate(cr, "ns")
	assert.Equal(t, uint32(50), result.Spec.Threshold)
	assert.Equal(t, "SECOND", result.Spec.Unit)
	assert.Equal(t, uint32(3), result.Spec.BurstMultiplier)
}

func TestBuildRateLimiterReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.Equal(t, uint32(100), result.Spec.Threshold)
}

func TestBuildRateLimiterCreate_XCNamespaceOverride(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "k8s-ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	result := buildRateLimiterCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildRateLimiterDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-rl", Namespace: "ns"},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}

	raw, err := buildRateLimiterDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasTotalNumber := spec["total_number"]
	_, hasUnit := spec["unit"]
	assert.True(t, hasTotalNumber)
	assert.True(t, hasUnit)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
```

- [ ] **Step 2: Run mapper tests to verify they fail**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run TestBuildRateLimiter -v -count=1`
Expected: FAIL — `buildRateLimiterCreate` undefined

- [ ] **Step 3: Write types file `api/v1alpha1/ratelimiter_types.go`**

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rl
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type RateLimiter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimiterSpec   `json:"spec,omitempty"`
	Status RateLimiterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type RateLimiterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimiter `json:"items"`
}

type RateLimiterSpec struct {
	Threshold       uint32  `json:"threshold"`
	Unit            string  `json:"unit"`
	BurstMultiplier *uint32 `json:"burstMultiplier,omitempty"`
}

type RateLimiterStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&RateLimiter{}, &RateLimiterList{})
}
```

- [ ] **Step 4: Run controller-gen to generate deepcopy**

Run: `controller-gen object paths="./api/..."`

- [ ] **Step 5: Write mapper `internal/controller/ratelimiter_mapper.go`**

```go
package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildRateLimiterCreate(cr *v1alpha1.RateLimiter, xcNamespace string) xcclient.XCRateLimiterCreate {
	return xcclient.XCRateLimiterCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapRateLimiterSpec(&cr.Spec),
	}
}

func buildRateLimiterReplace(cr *v1alpha1.RateLimiter, xcNamespace, resourceVersion string) xcclient.XCRateLimiterReplace {
	return xcclient.XCRateLimiterReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapRateLimiterSpec(&cr.Spec),
	}
}

func buildRateLimiterDesiredSpecJSON(cr *v1alpha1.RateLimiter, xcNamespace string) (json.RawMessage, error) {
	create := buildRateLimiterCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapRateLimiterSpec(spec *v1alpha1.RateLimiterSpec) xcclient.XCRateLimiterSpec {
	out := xcclient.XCRateLimiterSpec{
		Threshold: spec.Threshold,
		Unit:      spec.Unit,
	}
	if spec.BurstMultiplier != nil {
		out.BurstMultiplier = *spec.BurstMultiplier
	}
	return out
}
```

- [ ] **Step 6: Run mapper tests to verify they pass**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run TestBuildRateLimiter -v -count=1`
Expected: PASS

- [ ] **Step 7: Write controller test file `internal/controller/ratelimiter_controller_test.go`**

```go
package controller

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type fakeRateLimiterXCClient struct {
	fakeXCClient
	mu sync.Mutex

	rl         *xcclient.XCRateLimiter
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    xcclient.XCRateLimiterReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeRateLimiterXCClient) GetRateLimiter(_ context.Context, ns, name string) (*xcclient.XCRateLimiter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.rl == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.rl, nil
}

func (f *fakeRateLimiterXCClient) CreateRateLimiter(_ context.Context, ns string, rl xcclient.XCRateLimiterCreate) (*xcclient.XCRateLimiter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.XCRateLimiter{
		Metadata:       xcclient.ObjectMeta{Name: rl.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-rl"},
	}
	f.rl = result
	return result, nil
}

func (f *fakeRateLimiterXCClient) ReplaceRateLimiter(_ context.Context, ns, name string, rl xcclient.XCRateLimiterReplace) (*xcclient.XCRateLimiter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = rl
	return &xcclient.XCRateLimiter{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-rl"},
	}, nil
}

func (f *fakeRateLimiterXCClient) DeleteRateLimiter(_ context.Context, ns, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleteCalled = true
	f.deleteNS = ns
	f.deleteName = name
	return nil
}

func (f *fakeRateLimiterXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

func sampleRateLimiter(name, namespace string) *v1alpha1.RateLimiter {
	return &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}
}

func waitForRateLimiterCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.RateLimiter
		if err := testClient.Get(ctx, key, &cr); err == nil {
			cond := meta.FindStatusCondition(cr.Status.Conditions, condType)
			if cond != nil && cond.Status == wantStatus {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("timed out waiting for condition %s=%s on %s", condType, wantStatus, key)
}

func newRateLimiterReconciler(fake *fakeRateLimiterXCClient) *RateLimiterReconciler {
	return &RateLimiterReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startRateLimiterManager(t *testing.T, r *RateLimiterReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func TestRateLimiter_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-create", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-create", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.RateLimiter
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateRateLimiter to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
}

func TestRateLimiter_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{
		rl: &xcclient.XCRateLimiter{
			Metadata:       xcclient.ObjectMeta{Name: "rl-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"total_number":100,"unit":"MINUTE"}`),
		},
		needsUpdate: false,
	}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-uptodate", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-uptodate", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.RateLimiter
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionSynced)
	if cond == nil || cond.Reason != v1alpha1.ReasonUpToDate {
		t.Errorf("expected Synced reason UpToDate, got %v", cond)
	}

	fake.mu.Lock()
	replaced := fake.replaceCalled
	fake.mu.Unlock()

	if replaced {
		t.Error("expected ReplaceRateLimiter NOT to be called")
	}
}

func TestRateLimiter_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{
		rl: &xcclient.XCRateLimiter{
			Metadata:       xcclient.ObjectMeta{Name: "rl-update", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"total_number":50,"unit":"SECOND"}`),
		},
		needsUpdate: true,
	}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-update", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-update", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceRateLimiter to be called")
	}
	if replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace with resource_version rv-1, got %v", replaceArg.Metadata.ResourceVersion)
	}
}

func TestRateLimiter_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{getErr: xcclient.ErrAuth}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-auth-fail", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-auth-fail", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.RateLimiter
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestRateLimiter_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-delete", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-delete", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.RateLimiter
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.RateLimiter
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteRateLimiter to be called")
	}
}

func TestRateLimiter_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-orphan", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-orphan", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.RateLimiter
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.RateLimiter
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteRateLimiter NOT to be called with orphan policy")
	}
}

func TestRateLimiter_XCNamespaceAnnotation(t *testing.T) {
	setupSuite(t)
	fake := &fakeRateLimiterXCClient{}
	r := newRateLimiterReconciler(fake)
	startRateLimiterManager(t, r)

	cr := sampleRateLimiter("rl-xcns", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationXCNamespace: "custom-xc-ns"}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "rl-xcns", Namespace: "default"}
	waitForRateLimiterCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create with namespace custom-xc-ns, got %q", createNS)
	}
}

var _ xcclient.XCClient = (*fakeRateLimiterXCClient)(nil)
```

- [ ] **Step 8: Run controller tests to verify they fail**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run TestRateLimiter_ -v -count=1`
Expected: FAIL — `RateLimiterReconciler` undefined

- [ ] **Step 9: Write controller `internal/controller/ratelimiter_controller.go`**

```go
package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type RateLimiterReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=ratelimiters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=ratelimiters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=ratelimiters/finalizers,verbs=update

func (r *RateLimiterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ratelimiter", req.NamespacedName)

	var cr v1alpha1.RateLimiter
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, v1alpha1.FinalizerXCCleanup) {
		controllerutil.AddFinalizer(&cr, v1alpha1.FinalizerXCCleanup)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	xcNS := resolveRateLimiterXCNamespace(&cr)
	xc := r.ClientSet.Get()

	current, err := xc.GetRateLimiter(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current)
}

func (r *RateLimiterReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.RateLimiter, xc xcclient.XCClient, xcNS string) (ctrl.Result, error) {
	create := buildRateLimiterCreate(cr, xcNS)
	result, err := xc.CreateRateLimiter(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC rate limiter", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC rate limiter created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RateLimiterReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.RateLimiter, xc xcclient.XCClient, xcNS string, current *xcclient.XCRateLimiter) (ctrl.Result, error) {
	desiredJSON, err := buildRateLimiterDesiredSpecJSON(cr, xcNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC rate limiter is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildRateLimiterReplace(cr, xcNS, current.Metadata.ResourceVersion)
	result, err := xc.ReplaceRateLimiter(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC rate limiter", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC rate limiter updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RateLimiterReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.RateLimiter) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := resolveRateLimiterXCNamespace(cr)
		xc := r.ClientSet.Get()
		err := xc.DeleteRateLimiter(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC rate limiter", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC rate limiter", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RateLimiterReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.RateLimiter, err error, operation string) (ctrl.Result, error) {
	switch {
	case errors.Is(err, xcclient.ErrAuth):
		log.Error(err, "authentication failure — not retrying", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{}, nil

	case errors.Is(err, xcclient.ErrConflict):
		log.Info("conflict on XC API, requeueing immediately", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonConflict, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{Requeue: true}, nil

	case errors.Is(err, xcclient.ErrRateLimited):
		log.Info("rate limited by XC API, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonRateLimited, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil

	case errors.Is(err, xcclient.ErrServerError):
		log.Error(err, "XC API server error, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonServerError, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	default:
		log.Error(err, "XC API error, requeueing", "operation", operation)
		failReason := operationFailReason(operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, failReason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (r *RateLimiterReconciler) setStatus(cr *v1alpha1.RateLimiter, ready, synced bool, reason, message string, xcObj *xcclient.XCRateLimiter) {
	readyStatus := metav1.ConditionTrue
	if !ready {
		readyStatus = metav1.ConditionFalse
	}
	syncedStatus := metav1.ConditionTrue
	if !synced {
		syncedStatus = metav1.ConditionFalse
	}

	r.setCondition(cr, v1alpha1.ConditionReady, readyStatus, reason, message)
	r.setCondition(cr, v1alpha1.ConditionSynced, syncedStatus, reason, message)
	cr.Status.ObservedGeneration = cr.Generation

	if xcObj != nil {
		cr.Status.XCResourceVersion = xcObj.Metadata.ResourceVersion
		cr.Status.XCUID = xcObj.SystemMetadata.UID
		cr.Status.XCNamespace = xcObj.Metadata.Namespace
	}
}

func (r *RateLimiterReconciler) setCondition(cr *v1alpha1.RateLimiter, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func resolveRateLimiterXCNamespace(cr *v1alpha1.RateLimiter) string {
	if ns, ok := cr.Annotations[v1alpha1.AnnotationXCNamespace]; ok && ns != "" {
		return ns
	}
	return cr.Namespace
}

func (r *RateLimiterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RateLimiter{}).
		Complete(r)
}
```

- [ ] **Step 10: Generate CRD YAML**

Run: `controller-gen crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac`

- [ ] **Step 11: Run all controller tests**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run "TestRateLimiter_|TestBuildRateLimiter" -v -count=1`
Expected: All pass

- [ ] **Step 12: Write integration test `internal/controller/ratelimiter_integration_test.go`**

```go
package controller

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func waitForRateLimiterConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.RateLimiter {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.RateLimiter
		if err := testClient.Get(testCtx, key, &cr); err == nil {
			cond := meta.FindStatusCondition(cr.Status.Conditions, condType)
			if cond != nil && cond.Status == wantStatus {
				return &cr
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("timed out waiting for condition %s=%s on %s", condType, wantStatus, key)
	return nil
}

func TestRateLimiterIntegration_CreateLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &RateLimiterReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-rl-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleRateLimiter("int-rl", "int-rl-create")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "int-rl", Namespace: "int-rl-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NotNil(t, result)
	syncedCond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced)
	require.NotNil(t, syncedCond)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, syncedCond.Reason)

	requests := srv.Requests()
	var postFound bool
	for _, r := range requests {
		if r.Method == "POST" {
			postFound = true
		}
	}
	assert.True(t, postFound, "expected a POST request to the fake server")
}

func TestRateLimiterIntegration_DeleteLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &RateLimiterReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-rl-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleRateLimiter("int-rl-del", "int-rl-delete")
	require.NoError(t, testClient.Create(testCtx, cr))

	waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "int-rl-del", Namespace: "int-rl-delete"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.RateLimiter
		if err := testClient.Get(testCtx, types.NamespacedName{Name: "int-rl-del", Namespace: "int-rl-delete"}, &check); err != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	requests := srv.Requests()
	var deleteFound bool
	for _, r := range requests {
		if r.Method == "DELETE" {
			deleteFound = true
		}
	}
	assert.True(t, deleteFound, "expected a DELETE request to the fake server")
}

func TestRateLimiterIntegration_ErrorInjection429(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("POST", "rate_limiters", "int-rl-429", "rl-rate", testutil.ErrorSpec{
		StatusCode: 429,
		Body:       "rate limited",
		Times:      2,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &RateLimiterReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-rl-429"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleRateLimiter("rl-rate", "int-rl-429")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "rl-rate", Namespace: "int-rl-429"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	syncedCond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced)
	require.NotNil(t, syncedCond)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, syncedCond.Reason)
}

func TestRateLimiterIntegration_ErrorInjection401(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("GET", "rate_limiters", "int-rl-401", "rl-auth", testutil.ErrorSpec{
		StatusCode: 401,
		Body:       "unauthorized",
		Times:      0,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &RateLimiterReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-rl-401"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleRateLimiter("rl-auth", "int-rl-401")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "rl-auth", Namespace: "int-rl-401"}, v1alpha1.ConditionReady, metav1.ConditionFalse, 15*time.Second)
	require.NotNil(t, result)
	assert.Equal(t, v1alpha1.ReasonAuthFailure, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionReady).Reason)
}
```

- [ ] **Step 13: Run all RateLimiter tests**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run "TestRateLimiter" -v -count=1`
Expected: All pass

- [ ] **Step 14: Commit**

```bash
git add api/v1alpha1/ratelimiter_types.go api/v1alpha1/zz_generated.deepcopy.go internal/controller/ratelimiter_mapper.go internal/controller/ratelimiter_mapper_test.go internal/controller/ratelimiter_controller.go internal/controller/ratelimiter_controller_test.go internal/controller/ratelimiter_integration_test.go config/crd/ config/rbac/
git commit -m "Add RateLimiter CRD with controller, mapper, and full test suite"
```

---

## Task 3: HealthCheck CRD — Types, Mapper, Controller, Tests

Uses typed structs (not raw JSON) for probe config. XC client uses value types for Create/Replace. Response type stores spec as `RawSpec` (no parsed `Spec` field).

**Files:**
- Create: `api/v1alpha1/healthcheck_types.go`
- Create: `internal/controller/healthcheck_mapper.go`
- Create: `internal/controller/healthcheck_mapper_test.go`
- Create: `internal/controller/healthcheck_controller.go`
- Create: `internal/controller/healthcheck_controller_test.go`
- Create: `internal/controller/healthcheck_integration_test.go`

- [ ] **Step 1: Write mapper tests `internal/controller/healthcheck_mapper_test.go`**

```go
package controller

import (
	"encoding/json"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildHealthCheckCreate_HTTPProbe(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "default"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{
				Path:                "/healthz",
				UseHTTP2:            true,
				ExpectedStatusCodes: []string{"200", "201"},
			},
			Interval:           uint32Ptr(30),
			Timeout:            uint32Ptr(5),
			HealthyThreshold:   uint32Ptr(3),
			UnhealthyThreshold: uint32Ptr(2),
			JitterPercent:      uint32Ptr(10),
		},
	}

	result := buildHealthCheckCreate(cr, "default")
	assert.Equal(t, "my-hc", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	require.NotNil(t, result.Spec.HTTPHealthCheck)
	assert.Equal(t, "/healthz", result.Spec.HTTPHealthCheck.Path)
	assert.True(t, result.Spec.HTTPHealthCheck.UseHTTP2)
	assert.Equal(t, []string{"200", "201"}, result.Spec.HTTPHealthCheck.ExpectedStatusCodes)
	assert.Equal(t, uint32(30), result.Spec.Interval)
	assert.Equal(t, uint32(5), result.Spec.Timeout)
	assert.Equal(t, uint32(3), result.Spec.HealthyThreshold)
	assert.Equal(t, uint32(2), result.Spec.UnhealthyThreshold)
	assert.Equal(t, uint32(10), result.Spec.JitterPercent)
}

func TestBuildHealthCheckCreate_TCPProbe(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "tcp-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			TCPHealthCheck: &v1alpha1.TCPHealthCheckSpec{
				Send:    "PING",
				Receive: "PONG",
			},
		},
	}

	result := buildHealthCheckCreate(cr, "ns")
	require.NotNil(t, result.Spec.TCPHealthCheck)
	assert.Equal(t, "PING", result.Spec.TCPHealthCheck.Send)
	assert.Equal(t, "PONG", result.Spec.TCPHealthCheck.Receive)
	assert.Nil(t, result.Spec.HTTPHealthCheck)
}

func TestBuildHealthCheckCreate_OptionalFieldsOmitted(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "minimal-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/"},
		},
	}

	result := buildHealthCheckCreate(cr, "ns")
	assert.Equal(t, uint32(0), result.Spec.Interval)
	assert.Equal(t, uint32(0), result.Spec.Timeout)
	assert.Equal(t, uint32(0), result.Spec.HealthyThreshold)
	assert.Equal(t, uint32(0), result.Spec.UnhealthyThreshold)
}

func TestBuildHealthCheckReplace_IncludesResourceVersion(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/healthz"},
		},
	}

	result := buildHealthCheckReplace(cr, "ns", "rv-3")
	assert.Equal(t, "rv-3", result.Metadata.ResourceVersion)
}

func TestBuildHealthCheckDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "my-hc", Namespace: "ns"},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/healthz"},
			Interval:        uint32Ptr(30),
		},
	}

	raw, err := buildHealthCheckDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasHTTPHealthCheck := spec["http_health_check"]
	_, hasInterval := spec["interval"]
	assert.True(t, hasHTTPHealthCheck)
	assert.True(t, hasInterval)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}
```

- [ ] **Step 2: Run mapper tests to verify they fail**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run TestBuildHealthCheck -v -count=1`
Expected: FAIL

- [ ] **Step 3: Write types file `api/v1alpha1/healthcheck_types.go`**

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hc
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

type HealthCheckSpec struct {
	HTTPHealthCheck    *HTTPHealthCheckSpec `json:"httpHealthCheck,omitempty"`
	TCPHealthCheck     *TCPHealthCheckSpec  `json:"tcpHealthCheck,omitempty"`
	HealthyThreshold   *uint32             `json:"healthyThreshold,omitempty"`
	UnhealthyThreshold *uint32             `json:"unhealthyThreshold,omitempty"`
	Interval           *uint32             `json:"interval,omitempty"`
	Timeout            *uint32             `json:"timeout,omitempty"`
	JitterPercent      *uint32             `json:"jitterPercent,omitempty"`
}

type HTTPHealthCheckSpec struct {
	Path                string   `json:"path,omitempty"`
	UseHTTP2            bool     `json:"useHTTP2,omitempty"`
	ExpectedStatusCodes []string `json:"expectedStatusCodes,omitempty"`
}

type TCPHealthCheckSpec struct {
	Send    string `json:"send,omitempty"`
	Receive string `json:"receive,omitempty"`
}

type HealthCheckStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
```

- [ ] **Step 4: Run controller-gen**

Run: `controller-gen object paths="./api/..."`

- [ ] **Step 5: Write mapper `internal/controller/healthcheck_mapper.go`**

```go
package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildHealthCheckCreate(cr *v1alpha1.HealthCheck, xcNamespace string) xcclient.CreateHealthCheck {
	return xcclient.CreateHealthCheck{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapHealthCheckSpec(&cr.Spec),
	}
}

func buildHealthCheckReplace(cr *v1alpha1.HealthCheck, xcNamespace, resourceVersion string) xcclient.ReplaceHealthCheck {
	return xcclient.ReplaceHealthCheck{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapHealthCheckSpec(&cr.Spec),
	}
}

func buildHealthCheckDesiredSpecJSON(cr *v1alpha1.HealthCheck, xcNamespace string) (json.RawMessage, error) {
	create := buildHealthCheckCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapHealthCheckSpec(spec *v1alpha1.HealthCheckSpec) xcclient.HealthCheckSpec {
	var out xcclient.HealthCheckSpec

	if spec.HTTPHealthCheck != nil {
		out.HTTPHealthCheck = &xcclient.HTTPHealthCheck{
			Path:                spec.HTTPHealthCheck.Path,
			UseHTTP2:            spec.HTTPHealthCheck.UseHTTP2,
			ExpectedStatusCodes: spec.HTTPHealthCheck.ExpectedStatusCodes,
		}
	}

	if spec.TCPHealthCheck != nil {
		out.TCPHealthCheck = &xcclient.TCPHealthCheck{
			Send:    spec.TCPHealthCheck.Send,
			Receive: spec.TCPHealthCheck.Receive,
		}
	}

	if spec.HealthyThreshold != nil {
		out.HealthyThreshold = *spec.HealthyThreshold
	}
	if spec.UnhealthyThreshold != nil {
		out.UnhealthyThreshold = *spec.UnhealthyThreshold
	}
	if spec.Interval != nil {
		out.Interval = *spec.Interval
	}
	if spec.Timeout != nil {
		out.Timeout = *spec.Timeout
	}
	if spec.JitterPercent != nil {
		out.JitterPercent = *spec.JitterPercent
	}

	return out
}
```

- [ ] **Step 6: Run mapper tests**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run TestBuildHealthCheck -v -count=1`
Expected: PASS

- [ ] **Step 7: Write controller `internal/controller/healthcheck_controller.go`**

Same structure as RateLimiterReconciler with these differences:
- Type: `HealthCheckReconciler` operating on `v1alpha1.HealthCheck`
- XC client methods: `GetHealthCheck`, `CreateHealthCheck`, `ReplaceHealthCheck`, `DeleteHealthCheck`
- Create/Replace return values (not pointers): `buildHealthCheckCreate` returns `xcclient.CreateHealthCheck`, `buildHealthCheckReplace` returns `xcclient.ReplaceHealthCheck`
- setStatus takes `*xcclient.HealthCheck` — uses `xcObj.Metadata.ResourceVersion` and `xcObj.SystemMetadata.UID`
- RBAC resource: `healthchecks`
- Log values key: `"healthcheck"`
- Log messages: `"XC health check"`

```go
package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type HealthCheckReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=healthchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=healthchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=healthchecks/finalizers,verbs=update

func (r *HealthCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("healthcheck", req.NamespacedName)

	var cr v1alpha1.HealthCheck
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, v1alpha1.FinalizerXCCleanup) {
		controllerutil.AddFinalizer(&cr, v1alpha1.FinalizerXCCleanup)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	xcNS := resolveHealthCheckXCNamespace(&cr)
	xc := r.ClientSet.Get()

	current, err := xc.GetHealthCheck(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current)
}

func (r *HealthCheckReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.HealthCheck, xc xcclient.XCClient, xcNS string) (ctrl.Result, error) {
	create := buildHealthCheckCreate(cr, xcNS)
	result, err := xc.CreateHealthCheck(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC health check", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC health check created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HealthCheckReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.HealthCheck, xc xcclient.XCClient, xcNS string, current *xcclient.HealthCheck) (ctrl.Result, error) {
	desiredJSON, err := buildHealthCheckDesiredSpecJSON(cr, xcNS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC health check is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildHealthCheckReplace(cr, xcNS, current.Metadata.ResourceVersion)
	result, err := xc.ReplaceHealthCheck(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC health check", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC health check updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HealthCheckReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.HealthCheck) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := resolveHealthCheckXCNamespace(cr)
		xc := r.ClientSet.Get()
		err := xc.DeleteHealthCheck(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC health check", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC health check", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HealthCheckReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.HealthCheck, err error, operation string) (ctrl.Result, error) {
	switch {
	case errors.Is(err, xcclient.ErrAuth):
		log.Error(err, "authentication failure — not retrying", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{}, nil
	case errors.Is(err, xcclient.ErrConflict):
		log.Info("conflict on XC API, requeueing immediately", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonConflict, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{Requeue: true}, nil
	case errors.Is(err, xcclient.ErrRateLimited):
		log.Info("rate limited by XC API, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonRateLimited, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	case errors.Is(err, xcclient.ErrServerError):
		log.Error(err, "XC API server error, requeueing", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonServerError, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	default:
		log.Error(err, "XC API error, requeueing", "operation", operation)
		failReason := operationFailReason(operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, failReason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (r *HealthCheckReconciler) setStatus(cr *v1alpha1.HealthCheck, ready, synced bool, reason, message string, xcObj *xcclient.HealthCheck) {
	readyStatus := metav1.ConditionTrue
	if !ready {
		readyStatus = metav1.ConditionFalse
	}
	syncedStatus := metav1.ConditionTrue
	if !synced {
		syncedStatus = metav1.ConditionFalse
	}

	r.setCondition(cr, v1alpha1.ConditionReady, readyStatus, reason, message)
	r.setCondition(cr, v1alpha1.ConditionSynced, syncedStatus, reason, message)
	cr.Status.ObservedGeneration = cr.Generation

	if xcObj != nil {
		cr.Status.XCResourceVersion = xcObj.Metadata.ResourceVersion
		cr.Status.XCUID = xcObj.SystemMetadata.UID
		cr.Status.XCNamespace = xcObj.Metadata.Namespace
	}
}

func (r *HealthCheckReconciler) setCondition(cr *v1alpha1.HealthCheck, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func resolveHealthCheckXCNamespace(cr *v1alpha1.HealthCheck) string {
	if ns, ok := cr.Annotations[v1alpha1.AnnotationXCNamespace]; ok && ns != "" {
		return ns
	}
	return cr.Namespace
}

func (r *HealthCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.HealthCheck{}).
		Complete(r)
}
```

- [ ] **Step 8: Write controller tests `internal/controller/healthcheck_controller_test.go`**

Same pattern as RateLimiter controller tests. Define `fakeHealthCheckXCClient` (embeds `fakeXCClient`, overrides `GetHealthCheck`, `CreateHealthCheck`, `ReplaceHealthCheck`, `DeleteHealthCheck`, `ClientNeedsUpdate`). `sampleHealthCheck` returns a minimal HealthCheck CR with an HTTP probe. `waitForHealthCheckCondition`, `newHealthCheckReconciler`, `startHealthCheckManager` helpers. 7 tests: CreateWhenNotFound, SkipUpdateWhenUpToDate, UpdateWhenChanged, AuthFailureNoRequeue, DeletionCallsXCDelete, DeletionOrphanPolicy, XCNamespaceAnnotation.

Key differences from RateLimiter fake:
- `CreateHealthCheck` takes value type: `func (f *fakeHealthCheckXCClient) CreateHealthCheck(_ context.Context, ns string, hc xcclient.CreateHealthCheck) (*xcclient.HealthCheck, error)`
- `ReplaceHealthCheck` takes value type: `func (f *fakeHealthCheckXCClient) ReplaceHealthCheck(_ context.Context, ns, name string, hc xcclient.ReplaceHealthCheck) (*xcclient.HealthCheck, error)`
- Returns `*xcclient.HealthCheck` which has `RawSpec json.RawMessage` field (not parsed Spec)
- For the fake response in SkipUpdateWhenUpToDate, set `RawSpec` on the fake's stored object

- [ ] **Step 9: Write integration tests `internal/controller/healthcheck_integration_test.go`**

Same 4 tests as RateLimiter integration. Use `xcclient.ResourceHealthCheck` ("healthchecks") for error injection. Use `sampleHealthCheck` as CR factory.

- [ ] **Step 10: Generate CRD YAML and run all HealthCheck tests**

Run: `controller-gen object paths="./api/..." && controller-gen crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac`
Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run "TestHealthCheck|TestBuildHealthCheck" -v -count=1`
Expected: All pass

- [ ] **Step 11: Commit**

```bash
git add api/v1alpha1/healthcheck_types.go api/v1alpha1/zz_generated.deepcopy.go internal/controller/healthcheck_mapper.go internal/controller/healthcheck_mapper_test.go internal/controller/healthcheck_controller.go internal/controller/healthcheck_controller_test.go internal/controller/healthcheck_integration_test.go config/crd/ config/rbac/
git commit -m "Add HealthCheck CRD with controller, mapper, and full test suite"
```

---

## Task 4: ServicePolicy CRD — Types, Mapper, Controller, Tests

Simple spec: `Algo string` + `Rules []apiextensionsv1.JSON`. Rules are raw JSON arrays passed through to the XC API.

**Files:**
- Create: `api/v1alpha1/servicepolicy_types.go`
- Create: `internal/controller/servicepolicy_mapper.go`
- Create: `internal/controller/servicepolicy_mapper_test.go`
- Create: `internal/controller/servicepolicy_controller.go`
- Create: `internal/controller/servicepolicy_controller_test.go`
- Create: `internal/controller/servicepolicy_integration_test.go`

Follow exact same pattern as Tasks 2 and 3. Key specifics:

**Types:** `ServicePolicySpec` has `Algo string` and `Rules []apiextensionsv1.JSON`. Short name: `sp`. Import `apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"`.

**Mapper:** `mapServicePolicySpec` maps `Algo` directly, converts `Rules` from `[]apiextensionsv1.JSON` to `[]json.RawMessage` via loop:
```go
for _, rule := range spec.Rules {
    out.Rules = append(out.Rules, json.RawMessage(rule.Raw))
}
```

XC client uses pointer types: `*ServicePolicyCreate`, `*ServicePolicyReplace`.

**Controller:** `ServicePolicyReconciler`. RBAC resource: `servicepolicies`. XC methods: `GetServicePolicy`, `CreateServicePolicy`, `ReplaceServicePolicy`, `DeleteServicePolicy`. FakeXCServer error injection resource: `"service_policys"` (note the irregular plural from the XC API constant `ResourceServicePolicy`).

**Tests:** All 7 controller tests + 4 integration tests + mapper tests for `Algo`, `Rules` passthrough, and `buildServicePolicyDesiredSpecJSON`.

- [ ] **Step 1–11: Implement following the exact same step sequence as Task 2**

Mapper tests → fail → types → controller-gen → mapper → pass → controller tests → fail → controller → CRD gen → pass → integration tests → pass → commit.

```bash
git commit -m "Add ServicePolicy CRD with controller, mapper, and full test suite"
```

---

## Task 5: AppFirewall CRD — Types, Mapper, Controller, Tests

All fields are optional `*apiextensionsv1.JSON` — 7 OneOf groups. No required spec fields.

**Files:**
- Create: `api/v1alpha1/appfirewall_types.go`
- Create: `internal/controller/appfirewall_mapper.go`
- Create: `internal/controller/appfirewall_mapper_test.go`
- Create: `internal/controller/appfirewall_controller.go`
- Create: `internal/controller/appfirewall_controller_test.go`
- Create: `internal/controller/appfirewall_integration_test.go`

**Types:** `AppFirewallSpec` with 13 optional `*apiextensionsv1.JSON` fields matching the spec. Short name: `afw`.

**Mapper:** `mapAppFirewallSpec` — all fields use the OneOf passthrough pattern:
```go
if spec.Blocking != nil {
    out.Blocking = json.RawMessage(spec.Blocking.Raw)
}
```

XC client uses pointer types: `*AppFirewallCreate`, `*AppFirewallReplace`.

**Controller:** `AppFirewallReconciler`. RBAC resource: `appfirewalls`. XC methods: `GetAppFirewall`, `CreateAppFirewall`, `ReplaceAppFirewall`, `DeleteAppFirewall`. FakeXCServer error injection resource: `"app_firewalls"`.

**Sample CR for tests:** Minimal — just `Blocking: &apiextensionsv1.JSON{Raw: []byte("{}")}`.

**Tests:** All 7 controller tests + 4 integration tests + mapper tests for each OneOf group passthrough.

- [ ] **Step 1–11: Implement following the exact same step sequence as Task 2**

```bash
git commit -m "Add AppFirewall CRD with controller, mapper, and full test suite"
```

---

## Task 6: TCPLoadBalancer CRD — Types, Mapper, Controller, Tests

Medium complexity: required `Domains`, `ListenPort`, `OriginPools` (uses shared `RoutePool` type) + TLS OneOf + Advertise OneOf.

**Files:**
- Create: `api/v1alpha1/tcplb_types.go`
- Create: `internal/controller/tcplb_mapper.go`
- Create: `internal/controller/tcplb_mapper_test.go`
- Create: `internal/controller/tcplb_controller.go`
- Create: `internal/controller/tcplb_controller_test.go`
- Create: `internal/controller/tcplb_integration_test.go`

**Types:** `TCPLoadBalancerSpec` per spec. Short name: `tlb`. Uses `RoutePool` from `shared_types.go`.

**Mapper:** `mapTCPLoadBalancerSpec` maps:
- `Domains` directly
- `ListenPort` directly
- `OriginPools` via `mapRoutePool` helper:
```go
func mapRoutePool(rp *v1alpha1.RoutePool) xcclient.RoutePool {
    out := xcclient.RoutePool{
        Pool: mapObjectRef(&rp.Pool),
    }
    if rp.Weight != nil {
        out.Weight = *rp.Weight
    }
    if rp.Priority != nil {
        out.Priority = *rp.Priority
    }
    return out
}
```
- TLS and Advertise OneOf fields via raw JSON passthrough

XC client uses pointer types: `*TCPLoadBalancerCreate`, `*TCPLoadBalancerReplace`.

**Controller:** `TCPLoadBalancerReconciler`. RBAC resource: `tcploadbalancers`. XC methods: `GetTCPLoadBalancer`, `CreateTCPLoadBalancer`, `ReplaceTCPLoadBalancer`, `DeleteTCPLoadBalancer`. FakeXCServer error injection resource: `"tcp_loadbalancers"`.

**Mapper tests:** Basic fields, RoutePool mapping with weight/priority, TLS OneOf passthrough, Advertise OneOf passthrough, XC namespace override, `buildTCPLoadBalancerDesiredSpecJSON`.

- [ ] **Step 1–11: Implement following the exact same step sequence as Task 2**

```bash
git commit -m "Add TCPLoadBalancer CRD with controller, mapper, and full test suite"
```

---

## Task 7: HTTPLoadBalancer CRD — Types, Mapper, Controller, Tests

Most complex: required `Domains` and `DefaultRoutePools` + 12 OneOf groups + Routes array. Uses shared `RoutePool` type and `mapRoutePool` from Task 6.

**Files:**
- Create: `api/v1alpha1/httplb_types.go`
- Create: `internal/controller/httplb_mapper.go`
- Create: `internal/controller/httplb_mapper_test.go`
- Create: `internal/controller/httplb_controller.go`
- Create: `internal/controller/httplb_controller_test.go`
- Create: `internal/controller/httplb_integration_test.go`

**Types:** `HTTPLoadBalancerSpec` per spec — all fields exactly as listed in the design spec. Short name: `hlb`. Note the `AppFirewall *ObjectRef` field (not `*apiextensionsv1.JSON`).

**Mapper:** `mapHTTPLoadBalancerSpec` maps:
- `Domains` directly
- `DefaultRoutePools` via `mapRoutePool` (already defined in Task 6's mapper)
- `Routes` from `[]apiextensionsv1.JSON` to `json.RawMessage` (marshal the array)
- `AppFirewall` via `mapObjectRefPtr`
- All other OneOf fields via raw JSON passthrough pattern

```go
func mapHTTPLoadBalancerSpec(spec *v1alpha1.HTTPLoadBalancerSpec) xcclient.HTTPLoadBalancerSpec {
    var out xcclient.HTTPLoadBalancerSpec
    out.Domains = spec.Domains

    for _, rp := range spec.DefaultRoutePools {
        out.DefaultRoutePools = append(out.DefaultRoutePools, mapRoutePool(&rp))
    }

    if len(spec.Routes) > 0 {
        routesJSON, _ := json.Marshal(spec.Routes)
        out.Routes = routesJSON
    }

    // TLS OneOf
    if spec.HTTP != nil {
        out.HTTP = json.RawMessage(spec.HTTP.Raw)
    }
    if spec.HTTPS != nil {
        out.HTTPS = json.RawMessage(spec.HTTPS.Raw)
    }
    if spec.HTTPSAutoCert != nil {
        out.HTTPSAutoCert = json.RawMessage(spec.HTTPSAutoCert.Raw)
    }

    // WAF OneOf
    if spec.DisableWAF != nil {
        out.DisableWAF = json.RawMessage(spec.DisableWAF.Raw)
    }
    if spec.AppFirewall != nil {
        out.AppFirewall = mapObjectRefPtr(spec.AppFirewall)
    }

    // Bot defense OneOf
    if spec.DisableBotDefense != nil {
        out.DisableBotDefense = json.RawMessage(spec.DisableBotDefense.Raw)
    }
    if spec.BotDefense != nil {
        out.BotDefense = json.RawMessage(spec.BotDefense.Raw)
    }

    // API discovery OneOf
    if spec.DisableAPIDiscovery != nil {
        out.DisableAPIDiscovery = json.RawMessage(spec.DisableAPIDiscovery.Raw)
    }
    if spec.EnableAPIDiscovery != nil {
        out.EnableAPIDiscovery = json.RawMessage(spec.EnableAPIDiscovery.Raw)
    }

    // IP reputation OneOf
    if spec.DisableIPReputation != nil {
        out.DisableIPReputation = json.RawMessage(spec.DisableIPReputation.Raw)
    }
    if spec.EnableIPReputation != nil {
        out.EnableIPReputation = json.RawMessage(spec.EnableIPReputation.Raw)
    }

    // Rate limit OneOf
    if spec.DisableRateLimit != nil {
        out.DisableRateLimit = json.RawMessage(spec.DisableRateLimit.Raw)
    }
    if spec.RateLimit != nil {
        out.RateLimit = json.RawMessage(spec.RateLimit.Raw)
    }

    // Challenge OneOf
    if spec.NoChallenge != nil {
        out.NoChallenge = json.RawMessage(spec.NoChallenge.Raw)
    }
    if spec.JSChallenge != nil {
        out.JSChallenge = json.RawMessage(spec.JSChallenge.Raw)
    }
    if spec.CaptchaChallenge != nil {
        out.CaptchaChallenge = json.RawMessage(spec.CaptchaChallenge.Raw)
    }
    if spec.PolicyBasedChallenge != nil {
        out.PolicyBasedChallenge = json.RawMessage(spec.PolicyBasedChallenge.Raw)
    }

    // LB algorithm OneOf
    if spec.RoundRobin != nil {
        out.RoundRobin = json.RawMessage(spec.RoundRobin.Raw)
    }
    if spec.LeastActive != nil {
        out.LeastActive = json.RawMessage(spec.LeastActive.Raw)
    }
    if spec.Random != nil {
        out.Random = json.RawMessage(spec.Random.Raw)
    }
    if spec.SourceIPStickiness != nil {
        out.SourceIPStickiness = json.RawMessage(spec.SourceIPStickiness.Raw)
    }
    if spec.CookieStickiness != nil {
        out.CookieStickiness = json.RawMessage(spec.CookieStickiness.Raw)
    }
    if spec.RingHash != nil {
        out.RingHash = json.RawMessage(spec.RingHash.Raw)
    }

    // Advertise OneOf
    if spec.AdvertiseOnPublicDefaultVIP != nil {
        out.AdvertiseOnPublicDefaultVIP = json.RawMessage(spec.AdvertiseOnPublicDefaultVIP.Raw)
    }
    if spec.AdvertiseOnPublic != nil {
        out.AdvertiseOnPublic = json.RawMessage(spec.AdvertiseOnPublic.Raw)
    }
    if spec.AdvertiseCustom != nil {
        out.AdvertiseCustom = json.RawMessage(spec.AdvertiseCustom.Raw)
    }
    if spec.DoNotAdvertise != nil {
        out.DoNotAdvertise = json.RawMessage(spec.DoNotAdvertise.Raw)
    }

    // Service policies OneOf
    if spec.ServicePoliciesFromNamespace != nil {
        out.ServicePoliciesFromNamespace = json.RawMessage(spec.ServicePoliciesFromNamespace.Raw)
    }
    if spec.ActiveServicePolicies != nil {
        out.ActiveServicePolicies = json.RawMessage(spec.ActiveServicePolicies.Raw)
    }
    if spec.NoServicePolicies != nil {
        out.NoServicePolicies = json.RawMessage(spec.NoServicePolicies.Raw)
    }

    // User ID OneOf
    if spec.UserIDClientIP != nil {
        out.UserIDClientIP = json.RawMessage(spec.UserIDClientIP.Raw)
    }

    return out
}
```

XC client uses pointer types: `*HTTPLoadBalancerCreate`, `*HTTPLoadBalancerReplace`.

**Controller:** `HTTPLoadBalancerReconciler`. RBAC resource: `httploadbalancers`. XC methods: `GetHTTPLoadBalancer`, `CreateHTTPLoadBalancer`, `ReplaceHTTPLoadBalancer`, `DeleteHTTPLoadBalancer`. FakeXCServer error injection resource: `"http_loadbalancers"`.

**Mapper tests:** Required fields (Domains, DefaultRoutePools), AppFirewall ObjectRef, TLS OneOf, WAF OneOf, Advertise OneOf, `buildHTTPLoadBalancerDesiredSpecJSON`.

- [ ] **Step 1–11: Implement following the exact same step sequence as Task 2**

```bash
git commit -m "Add HTTPLoadBalancer CRD with controller, mapper, and full test suite"
```

---

## Task 8: Contract Tests for All 6 Resources

Append to existing `internal/controller/contract_test.go`. One function per resource, `//go:build contract` gated.

**Files:**
- Modify: `internal/controller/contract_test.go`

- [ ] **Step 1: Add contract tests for all 6 resources**

Each test follows the exact same pattern as `TestContract_OriginPoolCRDLifecycle`:
1. Call `setupSuite(t)` and `contractXCClient(t)`
2. Create resource-specific reconciler with `startManagerFor`
3. Create namespace
4. Clean up leftover from previous runs
5. Create CR with `AnnotationXCNamespace` pointing to `contractNamespace(t)`
6. Wait for `Ready=True`
7. Verify resource exists in XC API via direct client call
8. Delete CR and wait for cleanup
9. Verify resource is gone from XC API

Resource-specific details:
- **RateLimiter:** CR spec: `Threshold: 100, Unit: "MINUTE"`. XC resource: `rate_limiters`. Verify via `xcClient.GetRateLimiter`.
- **HealthCheck:** CR spec: `HTTPHealthCheck: {Path: "/health"}`. XC resource: `healthchecks`. Verify via `xcClient.GetHealthCheck`.
- **ServicePolicy:** CR spec: `Algo: "FIRST_MATCH"`. XC resource: `service_policys`. Verify via `xcClient.GetServicePolicy`.
- **AppFirewall:** CR spec: `Blocking: &apiextensionsv1.JSON{Raw: []byte("{}")}`. XC resource: `app_firewalls`. Verify via `xcClient.GetAppFirewall`.
- **TCPLoadBalancer:** CR spec: `Domains: ["tcp.test.com"], ListenPort: 443, OriginPools: [{Pool: {Name: "test-pool"}, Weight: uint32Ptr(1)}]`. XC resource: `tcp_loadbalancers`. Verify via `xcClient.GetTCPLoadBalancer`.
- **HTTPLoadBalancer:** CR spec: `Domains: ["http.test.com"], DefaultRoutePools: [{Pool: {Name: "test-pool"}, Weight: uint32Ptr(1)}], AdvertiseOnPublicDefaultVIP: &apiextensionsv1.JSON{Raw: []byte("{}")}`. XC resource: `http_loadbalancers`. Verify via `xcClient.GetHTTPLoadBalancer`.

- [ ] **Step 2: Verify contract tests compile (they skip without env vars)**

Run: `go test -tags=contract ./internal/controller/ -run TestContract -v -count=1`
Expected: All SKIP with "XC_TENANT_URL and XC_API_TOKEN required"

- [ ] **Step 3: Commit**

```bash
git add internal/controller/contract_test.go
git commit -m "Add contract test scaffolds for all 6 new CRDs"
```

---

## Task 9: Register All Controllers in main.go

**Files:**
- Modify: `cmd/main.go`

- [ ] **Step 1: Add 6 controller registrations after the OriginPool registration**

Add these blocks after the existing OriginPool registration (after line 124):

```go
	if err := (&controller.HTTPLoadBalancerReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("HTTPLoadBalancer"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "HTTPLoadBalancer")
		os.Exit(1)
	}

	if err := (&controller.TCPLoadBalancerReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("TCPLoadBalancer"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "TCPLoadBalancer")
		os.Exit(1)
	}

	if err := (&controller.AppFirewallReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("AppFirewall"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "AppFirewall")
		os.Exit(1)
	}

	if err := (&controller.HealthCheckReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("HealthCheck"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "HealthCheck")
		os.Exit(1)
	}

	if err := (&controller.ServicePolicyReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("ServicePolicy"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ServicePolicy")
		os.Exit(1)
	}

	if err := (&controller.RateLimiterReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("RateLimiter"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "RateLimiter")
		os.Exit(1)
	}
```

- [ ] **Step 2: Verify build**

Run: `go build -o /dev/null ./cmd/...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add cmd/main.go
git commit -m "Register all 6 new CRD controllers in operator manager"
```

---

## Task 10: Sample CRs, Manifests, and Final Cleanup

**Files:**
- Create: `config/samples/httplb.yaml`
- Create: `config/samples/tcplb.yaml`
- Create: `config/samples/appfirewall.yaml`
- Create: `config/samples/healthcheck.yaml`
- Create: `config/samples/servicepolicy.yaml`
- Create: `config/samples/ratelimiter.yaml`

- [ ] **Step 1: Create sample CRs**

Create each file with the content specified in the design spec (see "Sample CRs" section).

- [ ] **Step 2: Regenerate all manifests**

Run: `controller-gen object paths="./api/..." && controller-gen crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac`

Verify 7 CRD files in `config/crd/bases/` (1 existing OriginPool + 6 new).

- [ ] **Step 3: Run full test suite**

Run: `KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 4: Run gofmt and go vet**

Run: `gofmt -s -w . && go vet ./...`

- [ ] **Step 5: Commit**

```bash
git add config/samples/ config/crd/ config/rbac/ api/v1alpha1/zz_generated.deepcopy.go
git commit -m "Add sample CRs and regenerate CRD/RBAC manifests for all resources"
```
