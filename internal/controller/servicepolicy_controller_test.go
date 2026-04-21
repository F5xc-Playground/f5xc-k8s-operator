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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type fakeServicePolicyXCClient struct {
	fakeXCClient
	mu sync.Mutex

	sp         *xcclient.ServicePolicy
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    *xcclient.ServicePolicyReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeServicePolicyXCClient) GetServicePolicy(_ context.Context, ns, name string) (*xcclient.ServicePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.sp == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.sp, nil
}

func (f *fakeServicePolicyXCClient) CreateServicePolicy(_ context.Context, ns string, sp *xcclient.ServicePolicyCreate) (*xcclient.ServicePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.ServicePolicy{
		Metadata:       xcclient.ObjectMeta{Name: sp.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-sp"},
	}
	f.sp = result
	return result, nil
}

func (f *fakeServicePolicyXCClient) ReplaceServicePolicy(_ context.Context, ns, name string, sp *xcclient.ServicePolicyReplace) (*xcclient.ServicePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = sp
	return &xcclient.ServicePolicy{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-sp"},
	}, nil
}

func (f *fakeServicePolicyXCClient) DeleteServicePolicy(_ context.Context, ns, name string) error {
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

func (f *fakeServicePolicyXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

func sampleServicePolicy(name, namespace string) *v1alpha1.ServicePolicy {
	return &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      namespace,
			Algo:             "FIRST_MATCH",
			AllowAllRequests: &apiextensionsv1.JSON{Raw: []byte("{}")},
			AnyServer:        &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}
}

func waitForServicePolicyCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.ServicePolicy
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

func newServicePolicyReconciler(fake *fakeServicePolicyXCClient) *ServicePolicyReconciler {
	return &ServicePolicyReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startServicePolicyManager(t *testing.T, r *ServicePolicyReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func TestServicePolicy_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-create", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-create", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.ServicePolicy
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateServicePolicy to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
}

func TestServicePolicy_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{
		sp: &xcclient.ServicePolicy{
			Metadata:       xcclient.ObjectMeta{Name: "sp-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"algo":"FIRST_MATCH"}`),
		},
		needsUpdate: false,
	}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-uptodate", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-uptodate", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.ServicePolicy
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
		t.Error("expected ReplaceServicePolicy NOT to be called")
	}
}

func TestServicePolicy_UpdateWhenChanged(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{
		sp: &xcclient.ServicePolicy{
			Metadata:       xcclient.ObjectMeta{Name: "sp-update", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"algo":"DENY_ALL"}`),
		},
		needsUpdate: true,
	}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-update", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-update", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	fake.mu.Lock()
	replaced := fake.replaceCalled
	replaceArg := fake.replaceArg
	fake.mu.Unlock()

	if !replaced {
		t.Error("expected ReplaceServicePolicy to be called")
	}
	if replaceArg.Metadata.ResourceVersion != "rv-1" {
		t.Errorf("expected Replace with resource_version rv-1, got %v", replaceArg.Metadata.ResourceVersion)
	}
}

func TestServicePolicy_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{getErr: xcclient.ErrAuth}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-auth-fail", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-auth-fail", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.ServicePolicy
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestServicePolicy_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-delete", "default")
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-delete", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.ServicePolicy
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.ServicePolicy
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteServicePolicy to be called")
	}
}

func TestServicePolicy_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-orphan", "default")
	cr.Annotations = map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-orphan", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.ServicePolicy
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.ServicePolicy
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteServicePolicy NOT to be called with orphan policy")
	}
}

func TestServicePolicy_XCNamespaceSpec(t *testing.T) {
	setupSuite(t)
	fake := &fakeServicePolicyXCClient{}
	r := newServicePolicyReconciler(fake)
	startServicePolicyManager(t, r)

	cr := sampleServicePolicy("sp-xcns", "default")
	cr.Spec.XCNamespace = "custom-xc-ns"
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "sp-xcns", Namespace: "default"}
	waitForServicePolicyCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	createNS := fake.createNS
	fake.mu.Unlock()

	if createNS != "custom-xc-ns" {
		t.Errorf("expected Create with namespace custom-xc-ns, got %q", createNS)
	}
}

var _ xcclient.XCClient = (*fakeServicePolicyXCClient)(nil)
