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

type fakeAppFirewallXCClient struct {
	fakeXCClient
	mu sync.Mutex

	fw         *xcclient.AppFirewall
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    *xcclient.AppFirewallReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeAppFirewallXCClient) GetAppFirewall(_ context.Context, ns, name string) (*xcclient.AppFirewall, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.fw == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.fw, nil
}

func (f *fakeAppFirewallXCClient) CreateAppFirewall(_ context.Context, ns string, fw *xcclient.AppFirewallCreate) (*xcclient.AppFirewall, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.AppFirewall{
		Metadata:       xcclient.ObjectMeta{Name: fw.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-afw"},
	}
	f.fw = result
	return result, nil
}

func (f *fakeAppFirewallXCClient) ReplaceAppFirewall(_ context.Context, ns, name string, fw *xcclient.AppFirewallReplace) (*xcclient.AppFirewall, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = fw
	return &xcclient.AppFirewall{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-afw"},
	}, nil
}

func (f *fakeAppFirewallXCClient) DeleteAppFirewall(_ context.Context, ns, name string) error {
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

func (f *fakeAppFirewallXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

func waitForAppFirewallCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.AppFirewall
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

func newAppFirewallReconciler(fake *fakeAppFirewallXCClient) *AppFirewallReconciler {
	return &AppFirewallReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startAppFirewallManager(t *testing.T, r *AppFirewallReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func TestAppFirewall_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-create", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-create", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.AppFirewall
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateAppFirewall to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
}

func TestAppFirewall_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{
		fw: &xcclient.AppFirewall{
			Metadata:       xcclient.ObjectMeta{Name: "afw-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"blocking":{}}`),
		},
		needsUpdate: false,
	}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-uptodate", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-uptodate", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.AppFirewall
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
		t.Error("expected ReplaceAppFirewall NOT to be called")
	}
}

func TestAppFirewall_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{
		fw: &xcclient.AppFirewall{
			Metadata:       xcclient.ObjectMeta{Name: "afw-update", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"monitoring":{}}`),
		},
		needsUpdate: true,
	}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-update", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-update", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceAppFirewall to be called")
	}
	if replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace with resource_version rv-1, got %v", replaceArg.Metadata.ResourceVersion)
	}
}

func TestAppFirewall_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{getErr: xcclient.ErrAuth}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-auth-fail", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-auth-fail", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.AppFirewall
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestAppFirewall_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-delete", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-delete", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.AppFirewall
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.AppFirewall
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteAppFirewall to be called")
	}
}

func TestAppFirewall_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-orphan", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-orphan", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.AppFirewall
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.AppFirewall
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteAppFirewall NOT to be called with orphan policy")
	}
}

func TestAppFirewall_XCNamespaceSpec(t *testing.T) {
	setupSuite(t)
	fake := &fakeAppFirewallXCClient{}
	r := newAppFirewallReconciler(fake)
	startAppFirewallManager(t, r)

	cr := sampleAppFirewall("afw-xcns", "default")
	cr.Spec.XCNamespace = "custom-xc-ns"
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "afw-xcns", Namespace: "default"}
	waitForAppFirewallCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create with namespace custom-xc-ns, got %q", createNS)
	}
}

var _ xcclient.XCClient = (*fakeAppFirewallXCClient)(nil)
