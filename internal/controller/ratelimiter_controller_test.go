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
