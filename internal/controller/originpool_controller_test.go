package controller

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

// ---------------------------------------------------------------------------
// fakeXCClient — implements xcclient.XCClient for testing
// ---------------------------------------------------------------------------

type fakeXCClient struct {
	mu sync.Mutex

	// OriginPool state
	pool        *xcclient.OriginPool
	getErr      error
	createErr   error
	replaceErr  error
	deleteErr   error
	needsUpdate bool

	// Recorded calls
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    *xcclient.OriginPoolReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeXCClient) GetOriginPool(_ context.Context, ns, name string) (*xcclient.OriginPool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.pool == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.pool, nil
}

func (f *fakeXCClient) CreateOriginPool(_ context.Context, ns string, pool *xcclient.OriginPoolCreate) (*xcclient.OriginPool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.OriginPool{
		Metadata: xcclient.ObjectMeta{
			Name:            pool.Metadata.Name,
			Namespace:       ns,
			ResourceVersion: "rv-1",
		},
		SystemMetadata: xcclient.SystemMeta{
			UID: "uid-abc",
		},
	}
	f.pool = result
	return result, nil
}

func (f *fakeXCClient) ReplaceOriginPool(_ context.Context, ns, name string, pool *xcclient.OriginPoolReplace) (*xcclient.OriginPool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = pool
	result := &xcclient.OriginPool{
		Metadata: xcclient.ObjectMeta{
			Name:            name,
			Namespace:       ns,
			ResourceVersion: "rv-2",
		},
		SystemMetadata: xcclient.SystemMeta{
			UID: "uid-abc",
		},
	}
	return result, nil
}

func (f *fakeXCClient) DeleteOriginPool(_ context.Context, ns, name string) error {
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

func (f *fakeXCClient) ListOriginPools(_ context.Context, ns string) ([]*xcclient.OriginPool, error) {
	return nil, nil
}

func (f *fakeXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

// --- Stubs for the rest of the interface ---

func (f *fakeXCClient) CreateHTTPLoadBalancer(_ context.Context, ns string, lb *xcclient.HTTPLoadBalancerCreate) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) GetHTTPLoadBalancer(_ context.Context, ns, name string) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceHTTPLoadBalancer(_ context.Context, ns, name string, lb *xcclient.HTTPLoadBalancerReplace) (*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteHTTPLoadBalancer(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListHTTPLoadBalancers(_ context.Context, ns string) ([]*xcclient.HTTPLoadBalancer, error) {
	return nil, nil
}

func (f *fakeXCClient) CreateTCPLoadBalancer(_ context.Context, ns string, lb *xcclient.TCPLoadBalancerCreate) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) GetTCPLoadBalancer(_ context.Context, ns, name string) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceTCPLoadBalancer(_ context.Context, ns, name string, lb *xcclient.TCPLoadBalancerReplace) (*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteTCPLoadBalancer(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListTCPLoadBalancers(_ context.Context, ns string) ([]*xcclient.TCPLoadBalancer, error) {
	return nil, nil
}

func (f *fakeXCClient) CreateHealthCheck(_ context.Context, ns string, hc xcclient.CreateHealthCheck) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeXCClient) GetHealthCheck(_ context.Context, ns, name string) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceHealthCheck(_ context.Context, ns, name string, hc xcclient.ReplaceHealthCheck) (*xcclient.HealthCheck, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteHealthCheck(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListHealthChecks(_ context.Context, ns string) ([]*xcclient.HealthCheck, error) {
	return nil, nil
}

func (f *fakeXCClient) CreateAppFirewall(_ context.Context, ns string, fw *xcclient.AppFirewallCreate) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeXCClient) GetAppFirewall(_ context.Context, ns, name string) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceAppFirewall(_ context.Context, ns, name string, fw *xcclient.AppFirewallReplace) (*xcclient.AppFirewall, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteAppFirewall(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListAppFirewalls(_ context.Context, ns string) ([]*xcclient.AppFirewall, error) {
	return nil, nil
}

func (f *fakeXCClient) CreateServicePolicy(_ context.Context, ns string, sp *xcclient.ServicePolicyCreate) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeXCClient) GetServicePolicy(_ context.Context, ns, name string) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceServicePolicy(_ context.Context, ns, name string, sp *xcclient.ServicePolicyReplace) (*xcclient.ServicePolicy, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteServicePolicy(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListServicePolicies(_ context.Context, ns string) ([]*xcclient.ServicePolicy, error) {
	return nil, nil
}

func (f *fakeXCClient) CreateRateLimiter(_ context.Context, ns string, rl xcclient.XCRateLimiterCreate) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeXCClient) GetRateLimiter(_ context.Context, ns, name string) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeXCClient) ReplaceRateLimiter(_ context.Context, ns, name string, rl xcclient.XCRateLimiterReplace) (*xcclient.XCRateLimiter, error) {
	return nil, nil
}
func (f *fakeXCClient) DeleteRateLimiter(_ context.Context, ns, name string) error {
	return nil
}
func (f *fakeXCClient) ListRateLimiters(_ context.Context, ns string) ([]*xcclient.XCRateLimiter, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sampleOriginPool(name, namespace string) *v1alpha1.OriginPool {
	return &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{
					PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"},
				},
			},
			Port: 443,
		},
	}
}

func waitForCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.OriginPool
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

func newReconciler(fake *fakeXCClient) *OriginPoolReconciler {
	return &OriginPoolReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestReconcile_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{} // pool is nil → GetOriginPool returns ErrNotFound
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("create-test", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)
	waitForCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateOriginPool to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
	if updated.Status.XCResourceVersion == "" {
		t.Error("expected XCResourceVersion to be populated")
	}
}

func TestReconcile_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{
		pool: &xcclient.OriginPool{
			Metadata: xcclient.ObjectMeta{
				Name:            "uptodate-test",
				Namespace:       "default",
				ResourceVersion: "rv-1",
			},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"port":443}`),
		},
		needsUpdate: false,
	}
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("uptodate-test", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
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
		t.Error("expected ReplaceOriginPool NOT to be called when up to date")
	}
}

func TestReconcile_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{
		pool: &xcclient.OriginPool{
			Metadata: xcclient.ObjectMeta{
				Name:            "update-test",
				Namespace:       "default",
				ResourceVersion: "rv-1",
			},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"port":80}`),
		},
		needsUpdate: true,
	}
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("update-test", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceOriginPool to be called")
	}
	if replaceArg == nil || replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace called with resource_version rv-1, got %v", replaceArg)
	}
}

func TestReconcile_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{
		getErr: xcclient.ErrAuth,
	}
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("auth-fail-test", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.OriginPool
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %s", cond.Reason)
	}
}

func TestReconcile_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{} // starts with no pool → Create will be called first
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("delete-test", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	// Now delete the CR
	var latest v1alpha1.OriginPool
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	// Wait for CR to be fully removed (finalizer cleared)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.OriginPool
		err := testClient.Get(testCtx, key, &check)
		if err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	deleteNS := fake.deleteNS
	deleteName := fake.deleteName
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteOriginPool to be called")
	}
	if deleteNS != "default" {
		t.Errorf("expected delete namespace=default, got %s", deleteNS)
	}
	if deleteName != "delete-test" {
		t.Errorf("expected delete name=delete-test, got %s", deleteName)
	}
}

func TestReconcile_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("orphan-test", "default")
	cr.Annotations = map[string]string{
		v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan,
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.OriginPool
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	// Wait for CR to be fully removed
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.OriginPool
		err := testClient.Get(testCtx, key, &check)
		if err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteOriginPool NOT to be called with orphan policy")
	}
}

func TestReconcile_XCNamespaceAnnotation(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	cr := sampleOriginPool("xcns-test", "default")
	cr.Annotations = map[string]string{
		v1alpha1.AnnotationXCNamespace: "custom-xc-ns",
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: cr.Name, Namespace: cr.Namespace}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create called with namespace custom-xc-ns, got %q", createNS)
	}

	// Also check the status was updated with the XC namespace
	var updated v1alpha1.OriginPool
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	// The pool returned by fake has Namespace set to createNS
	if updated.Status.XCNamespace != "custom-xc-ns" {
		t.Errorf("expected status.xcNamespace=custom-xc-ns, got %q", updated.Status.XCNamespace)
	}
}

// Compile-time check: fakeXCClient must satisfy XCClient.
var _ xcclient.XCClient = (*fakeXCClient)(nil)
