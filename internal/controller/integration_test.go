package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func newRealClient(t *testing.T, serverURL string) *xcclient.Client {
	t.Helper()
	cfg := xcclient.Config{
		TenantURL: serverURL,
		APIToken:  "test-token",
	}
	c, err := xcclient.NewClient(cfg, logr.Discard(), prometheus.NewRegistry())
	require.NoError(t, err)
	c.SetBaseDelay(10 * time.Millisecond)
	return c
}

// waitForConditionResult polls until the named OriginPool has the given
// condition status, then returns the CR. It fails the test if the deadline
// expires.
func waitForConditionResult(t *testing.T, key types.NamespacedName, condType string, wantStatus metav1.ConditionStatus, timeout time.Duration) *v1alpha1.OriginPool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var cr v1alpha1.OriginPool
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

func TestIntegration_CreateLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleOriginPool("int-pool", "int-create")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "int-pool", Namespace: "int-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NotNil(t, result)
	// After create the reconciler immediately re-reconciles and transitions to
	// UpToDate. Accept either reason as proof of a successful create lifecycle.
	syncedCond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced)
	require.NotNil(t, syncedCond)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, syncedCond.Reason)

	requests := srv.Requests()
	var postFound bool
	for _, r := range requests {
		if r.Method == "POST" {
			postFound = true
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(r.Body, &body))
			_, hasSpec := body["spec"]
			assert.True(t, hasSpec)
		}
	}
	assert.True(t, postFound, "expected a POST request to the fake server")
}

func TestIntegration_DeleteLifecycle(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleOriginPool("int-del-pool", "int-delete")
	require.NoError(t, testClient.Create(testCtx, cr))

	waitForConditionResult(t, types.NamespacedName{Name: "int-del-pool", Namespace: "int-delete"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)

	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.OriginPool
		err := testClient.Get(testCtx, types.NamespacedName{Name: "int-del-pool", Namespace: "int-delete"}, &check)
		if err != nil {
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

func TestIntegration_ErrorInjection429(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("POST", "origin_pools", "int-429", "rate-pool", testutil.ErrorSpec{
		StatusCode: 429,
		Body:       "rate limited",
		Times:      2,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-429"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleOriginPool("rate-pool", "int-429")
	require.NoError(t, testClient.Create(testCtx, cr))

	// The client retries 429s internally, so this should eventually succeed.
	// After create succeeds the reconciler immediately re-reconciles and may
	// transition to UpToDate; accept either reason as proof of a successful create.
	result := waitForConditionResult(t, types.NamespacedName{Name: "rate-pool", Namespace: "int-429"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	syncedCond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced)
	require.NotNil(t, syncedCond)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, syncedCond.Reason)
}

func TestIntegration_ErrorInjection401(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	srv.InjectError("GET", "origin_pools", "int-401", "auth-pool", testutil.ErrorSpec{
		StatusCode: 401,
		Body:       "unauthorized",
		Times:      0,
	})

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "int-401"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := sampleOriginPool("auth-pool", "int-401")
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "auth-pool", Namespace: "int-401"}, v1alpha1.ConditionReady, metav1.ConditionFalse, 15*time.Second)
	require.NotNil(t, result)
	assert.Equal(t, v1alpha1.ReasonAuthFailure, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionReady).Reason)
}
