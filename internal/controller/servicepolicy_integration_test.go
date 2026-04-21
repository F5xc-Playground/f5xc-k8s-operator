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

func waitForServicePolicyConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.ServicePolicy {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.ServicePolicy
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

func TestServicePolicyIntegration_CreateLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &ServicePolicyReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-sp-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleServicePolicy("int-sp", "int-sp-create")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "int-sp", Namespace: "int-sp-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
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

func TestServicePolicyIntegration_DeleteLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &ServicePolicyReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-sp-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleServicePolicy("int-sp-del", "int-sp-delete")
	require.NoError(t, testClient.Create(testCtx, cr))

	waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "int-sp-del", Namespace: "int-sp-delete"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.ServicePolicy
		if err := testClient.Get(testCtx, types.NamespacedName{Name: "int-sp-del", Namespace: "int-sp-delete"}, &check); err != nil {
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

func TestServicePolicyIntegration_ErrorInjection429(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("POST", "service_policys", "int-sp-429", "sp-rate", testutil.ErrorSpec{
		StatusCode: 429,
		Body:       "rate limited",
		Times:      2,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &ServicePolicyReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-sp-429"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleServicePolicy("sp-rate", "int-sp-429")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "sp-rate", Namespace: "int-sp-429"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	syncedCond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced)
	require.NotNil(t, syncedCond)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, syncedCond.Reason)
}

func TestServicePolicyIntegration_ErrorInjection401(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("GET", "service_policys", "int-sp-401", "sp-auth", testutil.ErrorSpec{
		StatusCode: 401,
		Body:       "unauthorized",
		Times:      0,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &ServicePolicyReconciler{Log: logr.Discard(), ClientSet: cs}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-sp-401"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleServicePolicy("sp-auth", "int-sp-401")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "sp-auth", Namespace: "int-sp-401"}, v1alpha1.ConditionReady, metav1.ConditionFalse, 15*time.Second)
	require.NotNil(t, result)
	assert.Equal(t, v1alpha1.ReasonAuthFailure, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionReady).Reason)
}
