//go:build contract

package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func contractXCClient(t *testing.T) *xcclient.Client {
	t.Helper()
	tenantURL := os.Getenv("XC_TENANT_URL")
	apiToken := os.Getenv("XC_API_TOKEN")
	if tenantURL == "" || apiToken == "" {
		t.Skip("XC_TENANT_URL and XC_API_TOKEN required for contract tests")
	}

	log := funcr.New(func(prefix, args string) {
		t.Logf("%s: %s", prefix, args)
	}, funcr.Options{Verbosity: 1})

	cfg := xcclient.Config{
		TenantURL: tenantURL,
		APIToken:  apiToken,
	}
	c, err := xcclient.NewClient(cfg, log, prometheus.NewRegistry())
	require.NoError(t, err)
	return c
}

func contractNamespace(t *testing.T) string {
	t.Helper()
	ns := os.Getenv("XC_TEST_NAMESPACE")
	if ns == "" {
		ns = "operator-test"
	}
	return ns
}

func TestContract_OriginPoolCRDLifecycle(t *testing.T) {
	setupSuite(t)

	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	cs := xcclientset.New(xcClient)

	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-op"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteOriginPool(context.Background(), xcNS, "contract-pool")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-pool",
			Namespace: "contract-op",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "203.0.113.10"}},
			},
			Port: 443,
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForConditionResult(t, types.NamespacedName{Name: "contract-pool", Namespace: "contract-op"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	assert.Equal(t, v1alpha1.ReasonCreateSucceeded, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	pool, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool")
	require.NoError(t, err)
	assert.Equal(t, 443, pool.Spec.Port)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_RateLimiterCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &RateLimiterReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-rl"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteRateLimiter(context.Background(), xcNS, "contract-rl")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-rl",
			Namespace: "contract-rl",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.RateLimiterSpec{
			Threshold: 100,
			Unit:      "MINUTE",
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "contract-rl", Namespace: "contract-rl"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	rl, err := xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl")
	require.NoError(t, err)
	assert.NotNil(t, rl)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_HealthCheckCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &HealthCheckReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-hc"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteHealthCheck(context.Background(), xcNS, "contract-hc")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hc",
			Namespace: "contract-hc",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.HealthCheckSpec{
			HTTPHealthCheck: &v1alpha1.HTTPHealthCheckSpec{Path: "/health"},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForHealthCheckConditionResult(t, types.NamespacedName{Name: "contract-hc", Namespace: "contract-hc"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	hc, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc")
	require.NoError(t, err)
	assert.NotNil(t, hc.RawSpec)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_ServicePolicyCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &ServicePolicyReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-sp"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteServicePolicy(context.Background(), xcNS, "contract-sp")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-sp",
			Namespace: "contract-sp",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.ServicePolicySpec{
			Algo: "FIRST_MATCH",
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "contract-sp", Namespace: "contract-sp"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	sp, err := xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp")
	require.NoError(t, err)
	assert.Equal(t, "FIRST_MATCH", sp.Spec.Algo)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_AppFirewallCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &AppFirewallReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-afw"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteAppFirewall(context.Background(), xcNS, "contract-afw")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-afw",
			Namespace: "contract-afw",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForAppFirewallConditionResult(t, types.NamespacedName{Name: "contract-afw", Namespace: "contract-afw"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	obj, err := xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_TCPLoadBalancerCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &TCPLoadBalancerReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-tlb"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteTCPLoadBalancer(context.Background(), xcNS, "contract-tlb")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-tlb",
			Namespace: "contract-tlb",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.TCPLoadBalancerSpec{
			Domains:    []string{"tcp.test.com"},
			ListenPort: 443,
			OriginPools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "test-pool"}, Weight: uint32Ptr(1)},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForTCPLoadBalancerConditionResult(t, types.NamespacedName{Name: "contract-tlb", Namespace: "contract-tlb"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	tlb, err := xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb")
	require.NoError(t, err)
	assert.NotEmpty(t, tlb.Spec.Domains)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_HTTPLoadBalancerCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &HTTPLoadBalancerReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-hlb"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hlb",
			Namespace: "contract-hlb",
			Annotations: map[string]string{
				v1alpha1.AnnotationXCNamespace: xcNS,
			},
		},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			Domains: []string{"http.test.com"},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "test-pool"}, Weight: uint32Ptr(1)},
			},
			AdvertiseOnPublicDefaultVIP: &apiextensionsv1.JSON{Raw: []byte("{}")},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Wait for Ready=True.
	result := waitForHTTPLoadBalancerConditionResult(t, types.NamespacedName{Name: "contract-hlb", Namespace: "contract-hlb"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	// Verify it exists in XC API.
	hlb, err := xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb")
	require.NoError(t, err)
	assert.NotEmpty(t, hlb.Spec.Domains)

	// Delete the CR and wait for cleanup.
	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	_, err = xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}
