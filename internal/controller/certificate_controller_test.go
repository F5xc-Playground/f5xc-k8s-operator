package controller

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type fakeCertificateXCClient struct {
	fakeXCClient
	mu sync.Mutex

	cert       *xcclient.Certificate
	getErr     error
	createErr  error
	replaceErr error
	deleteErr  error

	needsUpdate   bool
	createCalled  bool
	replaceCalled bool
	deleteCalled  bool
	replaceArg    *xcclient.CertificateReplace
	deleteNS      string
	deleteName    string
	createNS      string
}

func (f *fakeCertificateXCClient) GetCertificate(_ context.Context, ns, name string) (*xcclient.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.cert == nil {
		return nil, xcclient.ErrNotFound
	}
	return f.cert, nil
}

func (f *fakeCertificateXCClient) CreateCertificate(_ context.Context, ns string, cert *xcclient.CertificateCreate) (*xcclient.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createCalled = true
	f.createNS = ns
	result := &xcclient.Certificate{
		Metadata:       xcclient.ObjectMeta{Name: cert.Metadata.Name, Namespace: ns, ResourceVersion: "rv-1"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-cert"},
	}
	f.cert = result
	return result, nil
}

func (f *fakeCertificateXCClient) ReplaceCertificate(_ context.Context, ns, name string, cert *xcclient.CertificateReplace) (*xcclient.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	f.replaceCalled = true
	f.replaceArg = cert
	return &xcclient.Certificate{
		Metadata:       xcclient.ObjectMeta{Name: name, Namespace: ns, ResourceVersion: "rv-2"},
		SystemMetadata: xcclient.SystemMeta{UID: "uid-cert"},
	}, nil
}

func (f *fakeCertificateXCClient) DeleteCertificate(_ context.Context, ns, name string) error {
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

func (f *fakeCertificateXCClient) ClientNeedsUpdate(current, desired json.RawMessage) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.needsUpdate, nil
}

func waitForCertificateCondition(t *testing.T, ctx context.Context, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var cr v1alpha1.Certificate
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

func newCertificateReconciler(fake *fakeCertificateXCClient) *CertificateReconciler {
	return &CertificateReconciler{
		Log:       logr.Discard(),
		ClientSet: xcclientset.New(fake),
	}
}

func startCertificateManager(t *testing.T, r *CertificateReconciler) {
	startManagerFor(t, func(mgr ctrl.Manager) error {
		r.Client = mgr.GetClient()
		return r.SetupWithManager(mgr)
	})
}

func createTLSSecret(t *testing.T, name, namespace string) {
	t.Helper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"),
		},
	}
	if err := testClient.Create(testCtx, secret); err != nil {
		t.Fatalf("creating TLS secret: %v", err)
	}
}

func TestCertificate_CreateWhenNotFound(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	createTLSSecret(t, "tls-create", "default")

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-create", Namespace: "default"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "tls-create"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-create", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.Certificate
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting updated CR: %v", err)
	}

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()

	if !created {
		t.Error("expected CreateCertificate to be called")
	}
	if updated.Status.XCUID == "" {
		t.Error("expected XCUID to be populated")
	}
}

func TestCertificate_SkipUpdateWhenUpToDate(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{
		cert: &xcclient.Certificate{
			Metadata:       xcclient.ObjectMeta{Name: "cert-uptodate", Namespace: "default", ResourceVersion: "rv-1"},
			SystemMetadata: xcclient.SystemMeta{UID: "uid-1"},
			RawSpec:        json.RawMessage(`{"certificate_url":"string:///dGVzdA=="}`),
		},
		needsUpdate: false,
	}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	createTLSSecret(t, "tls-uptodate", "default")

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-uptodate", Namespace: "default"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "tls-uptodate"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-uptodate", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var updated v1alpha1.Certificate
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
		t.Error("expected ReplaceCertificate NOT to be called")
	}
}

func TestCertificate_AuthFailureNoRequeue(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{getErr: xcclient.ErrAuth}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	createTLSSecret(t, "tls-authfail", "default")

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-auth-fail", Namespace: "default"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "tls-authfail"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-auth-fail", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.Certificate
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != v1alpha1.ReasonAuthFailure {
		t.Errorf("expected AuthFailure reason, got %v", cond)
	}
}

func TestCertificate_DeletionCallsXCDelete(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	createTLSSecret(t, "tls-delete", "default")

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-delete", Namespace: "default"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "tls-delete"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-delete", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.Certificate
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.Certificate
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if !deleted {
		t.Error("expected DeleteCertificate to be called")
	}
}

func TestCertificate_DeletionOrphanPolicy(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	createTLSSecret(t, "tls-orphan", "default")

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cert-orphan",
			Namespace:   "default",
			Annotations: map[string]string{v1alpha1.AnnotationDeletionPolicy: v1alpha1.DeletionPolicyOrphan},
		},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "tls-orphan"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-orphan", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionSynced, metav1.ConditionTrue)

	var latest v1alpha1.Certificate
	if err := testClient.Get(testCtx, key, &latest); err != nil {
		t.Fatalf("getting CR before delete: %v", err)
	}
	if err := testClient.Delete(testCtx, &latest); err != nil {
		t.Fatalf("deleting CR: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.Certificate
		if err := testClient.Get(testCtx, key, &check); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	fake.mu.Lock()
	deleted := fake.deleteCalled
	fake.mu.Unlock()

	if deleted {
		t.Error("expected DeleteCertificate NOT to be called with orphan policy")
	}
}

func TestCertificate_SecretReadFailure(t *testing.T) {
	setupSuite(t)
	fake := &fakeCertificateXCClient{}
	r := newCertificateReconciler(fake)
	startCertificateManager(t, r)

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-no-secret", Namespace: "default"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "default",
			SecretRef:   v1alpha1.SecretRef{Name: "nonexistent-secret"},
		},
	}
	if err := testClient.Create(testCtx, cr); err != nil {
		t.Fatalf("creating CR: %v", err)
	}

	key := types.NamespacedName{Name: "cert-no-secret", Namespace: "default"}
	waitForCertificateCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.Certificate
	if err := testClient.Get(testCtx, key, &updated); err != nil {
		t.Fatalf("getting CR: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != "SecretReadFailed" {
		t.Errorf("expected SecretReadFailed reason, got %v", cond)
	}
}

var _ xcclient.XCClient = (*fakeCertificateXCClient)(nil)
