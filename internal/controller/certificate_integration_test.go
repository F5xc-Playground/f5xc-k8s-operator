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

func waitForCertificateConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.Certificate {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.Certificate
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

func TestCertificateIntegration_CreateLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &CertificateReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-cert-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "int-tls", Namespace: "int-cert-create"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"),
		},
	}
	require.NoError(t, testClient.Create(testCtx, secret))

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "int-cert", Namespace: "int-cert-create"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "int-cert-create",
			SecretRef:   v1alpha1.SecretRef{Name: "int-tls"},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForCertificateConditionResult(t, types.NamespacedName{Name: "int-cert", Namespace: "int-cert-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
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

func TestCertificateIntegration_DeleteLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &CertificateReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-cert-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "int-tls-del", Namespace: "int-cert-delete"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----"),
			"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"),
		},
	}
	require.NoError(t, testClient.Create(testCtx, secret))

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "int-cert-del", Namespace: "int-cert-delete"},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace: "int-cert-delete",
			SecretRef:   v1alpha1.SecretRef{Name: "int-tls-del"},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	waitForCertificateConditionResult(t, types.NamespacedName{Name: "int-cert-del", Namespace: "int-cert-delete"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.Certificate
		if err := testClient.Get(testCtx, types.NamespacedName{Name: "int-cert-del", Namespace: "int-cert-delete"}, &check); err != nil {
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
