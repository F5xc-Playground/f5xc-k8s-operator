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

type fakeHealthCheckXCClient struct {
	fakeXCClient
	mu sync.Mutex

	hc         *xcclient.HealthCheck
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    xcclient.ReplaceHealthCheck
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeHealthCheckXCClient) GetHealthCheck(_ context.Context, ns, name string) (*xcclient.HealthCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.hc == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.hc, nil
}

func (f *fakeHealthCheckXCClient) CreateHealthCheck(_ context.Context, ns string, hc xcclient.CreateHealthCheck) (*xcclient.HealthCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.HealthCheck{
		Metadata:       xcclient.ObjectMeta{Name: hc.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-hc"},
	}
	f.hc = result
	return result, nil
}

func (f *fakeHealthCheckXCClient) ReplaceHealthCheck(_ context.Context, ns, name string, hc xcclient.ReplaceHealthCheck) (*xcclient.HealthCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = hc
	return &xcclient.HealthCheck{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-hc"},
	}, nil
}

func (f *fakeHealthCheckXCClient) DeleteHealthCheck(_ context.Context, ns, name string) error {
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

func (f *fakeHealthCheckXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

func sampleHealthCheck(name, namespace string) *v1alpha1.HealthCheck {
	return &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.HealthCheckSpec{
			XCNamespace:     namespace,
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/healthz"},
		},
	}
}

func waitForHealthCheckCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.HealthCheck
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

func newHealthCheckReconciler(fake *fakeHealthCheckXCClient) *HealthCheckReconciler {
	return &HealthCheckReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startHealthCheckManager(t *testing.T, r *HealthCheckReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func TestHealthCheck_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-create", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-create", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.HealthCheck
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateHealthCheck to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
}

func TestHealthCheck_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{
		hc: &xcclient.HealthCheck{
			Metadata:       xcclient.ObjectMeta{Name: "hc-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"http_health_check":{"path":"/healthz"}}`),
		},
		needsUpdate: false,
	}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-uptodate", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-uptodate", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.HealthCheck
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
		t.Error("expected ReplaceHealthCheck NOT to be called")
	}
}

func TestHealthCheck_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{
		hc: &xcclient.HealthCheck{
			Metadata:       xcclient.ObjectMeta{Name: "hc-update", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"http_health_check":{"path":"/old"}}`),
		},
		needsUpdate: true,
	}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-update", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-update", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceHealthCheck to be called")
	}
	if replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace with resource_version rv-1, got %v", replaceArg.Metadata.ResourceVersion)
	}
}

func TestHealthCheck_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{getErr: xcclient.ErrAuth}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-auth-fail", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-auth-fail", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.HealthCheck
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestHealthCheck_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-delete", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-delete", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.HealthCheck
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.HealthCheck
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteHealthCheck to be called")
	}
}

func TestHealthCheck_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-orphan", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-orphan", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.HealthCheck
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.HealthCheck
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteHealthCheck NOT to be called with orphan policy")
	}
}

func TestHealthCheck_XCNamespaceSpec(t *testing.T) {
	setupSuite(t)
	fake := &fakeHealthCheckXCClient{}
	r := newHealthCheckReconciler(fake)
	startHealthCheckManager(t, r)

	cr := sampleHealthCheck("hc-xcns", "default")
	cr.Spec.XCNamespace = "custom-xc-ns"
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "hc-xcns", Namespace: "default"}
	waitForHealthCheckCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create with namespace custom-xc-ns, got %q", createNS)
	}
}

var _ xcclient.XCClient = (*fakeHealthCheckXCClient)(nil)
