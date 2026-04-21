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

// ---------------------------------------------------------------------------
// fakeTCPLoadBalancerXCClient
// ---------------------------------------------------------------------------

type fakeTCPLoadBalancerXCClient struct {
	fakeXCClient
	mu sync.Mutex

	tlb        *xcclient.TCPLoadBalancer
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    *xcclient.TCPLoadBalancerReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeTCPLoadBalancerXCClient) GetTCPLoadBalancer(_ context.Context, ns, name string) (*xcclient.TCPLoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.tlb == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.tlb, nil
}

func (f *fakeTCPLoadBalancerXCClient) CreateTCPLoadBalancer(_ context.Context, ns string, lb *xcclient.TCPLoadBalancerCreate) (*xcclient.TCPLoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.TCPLoadBalancer{
		Metadata:       xcclient.ObjectMeta{Name: lb.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-tlb"},
	}
	f.tlb = result
	return result, nil
}

func (f *fakeTCPLoadBalancerXCClient) ReplaceTCPLoadBalancer(_ context.Context, ns, name string, lb *xcclient.TCPLoadBalancerReplace) (*xcclient.TCPLoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = lb
	return &xcclient.TCPLoadBalancer{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-tlb"},
	}, nil
}

func (f *fakeTCPLoadBalancerXCClient) DeleteTCPLoadBalancer(_ context.Context, ns, name string) error {
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

func (f *fakeTCPLoadBalancerXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTCPLoadBalancerReconciler(fake *fakeTCPLoadBalancerXCClient) *TCPLoadBalancerReconciler {
	return &TCPLoadBalancerReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startTCPLoadBalancerManager(t *testing.T, r *TCPLoadBalancerReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func waitForTCPLoadBalancerCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.TCPLoadBalancer
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTCPLoadBalancer_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-create", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-create", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.TCPLoadBalancer
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateTCPLoadBalancer to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
	if updated.Status.XCResourceVersion == "" {
		t.Error("expected XCResourceVersion to be populated")
	}
}

func TestTCPLoadBalancer_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{
		tlb: &xcclient.TCPLoadBalancer{
			Metadata:       xcclient.ObjectMeta{Name: "tlb-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"listen_port":443}`),
		},
		needsUpdate: false,
	}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-uptodate", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-uptodate", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.TCPLoadBalancer
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
		t.Error("expected ReplaceTCPLoadBalancer NOT to be called when up to date")
	}
}

func TestTCPLoadBalancer_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{
		tlb: &xcclient.TCPLoadBalancer{
			Metadata:       xcclient.ObjectMeta{Name: "tlb-update", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"listen_port":80}`),
		},
		needsUpdate: true,
	}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-update", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-update", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceTCPLoadBalancer to be called")
	}
	if replaceArg == nil || replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace called with resource_version rv-1, got %v", replaceArg)
	}
}

func TestTCPLoadBalancer_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{getErr: xcclient.ErrAuth}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-auth-fail", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-auth-fail", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.TCPLoadBalancer
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestTCPLoadBalancer_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-delete", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-delete", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.TCPLoadBalancer
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.TCPLoadBalancer
		if err := testClient.Get(testCtx, key, &check); err != nil {
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
		t.Error("expected DeleteTCPLoadBalancer to be called")
	}
	if deleteNS != "default" {
		t.Errorf("expected delete namespace=default, got %s", deleteNS)
	}
	if deleteName != "tlb-delete" {
		t.Errorf("expected delete name=tlb-delete, got %s", deleteName)
	}
}

func TestTCPLoadBalancer_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-orphan", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-orphan", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.TCPLoadBalancer
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.TCPLoadBalancer
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteTCPLoadBalancer NOT to be called with orphan policy")
	}
}

func TestTCPLoadBalancer_XCNamespaceAnnotation(t *testing.T) {
	setupSuite(t)
	fake := &fakeTCPLoadBalancerXCClient{}
	r := newTCPLoadBalancerReconciler(fake)
	startTCPLoadBalancerManager(t, r)

	cr := sampleTCPLoadBalancer("tlb-xcns", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationXCNamespace: "custom-xc-ns"}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "tlb-xcns", Namespace: "default"}
	waitForTCPLoadBalancerCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create called with namespace custom-xc-ns, got %q", createNS)
	}

	var updated v1alpha1.TCPLoadBalancer
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}
	if updated.Status.XCNamespace != "custom-xc-ns" {
		t.Errorf("expected status.xcNamespace=custom-xc-ns, got %q", updated.Status.XCNamespace)
	}
}

var _ xcclient.XCClient = (*fakeTCPLoadBalancerXCClient)(nil)
