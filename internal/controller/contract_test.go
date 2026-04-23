//go:build contract

package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
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
		},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: xcNS,
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
		},
		Spec: v1alpha1.RateLimiterSpec{
			XCNamespace: xcNS,
			Threshold:   100,
			Unit:        "MINUTE",
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
		},
		Spec: v1alpha1.HealthCheckSpec{
			XCNamespace:     xcNS,
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
		},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:      xcNS,
			AllowAllRequests: &v1alpha1.EmptyObject{},
			AnyServer:        &v1alpha1.EmptyObject{},
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
	assert.NotNil(t, sp.Spec.AllowAllRequests)

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
		},
		Spec: v1alpha1.AppFirewallSpec{
			XCNamespace: xcNS,
			Blocking:    &v1alpha1.EmptyObject{},
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

	contractEnsureOriginPool(t, xcClient, xcNS, "contract-pool-tlb")

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteTCPLoadBalancer(context.Background(), xcNS, "contract-tlb")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-tlb",
			Namespace: "contract-tlb",
		},
		Spec: v1alpha1.TCPLoadBalancerSpec{
			XCNamespace: xcNS,
			Domains:     []string{"contract-tlb.k8s-op-test.example.com"},
			ListenPort:  443,
			OriginPools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "contract-pool-tlb"}, Weight: uint32Ptr(1)},
			},
			DoNotAdvertise: &v1alpha1.EmptyObject{},
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

	contractEnsureOriginPool(t, xcClient, xcNS, "contract-pool-hlb")

	// Clean up any leftover from a previous run.
	_ = xcClient.DeleteHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hlb",
			Namespace: "contract-hlb",
		},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: xcNS,
			Domains:     []string{"contract-hlb.k8s-op-test.example.com"},
			HTTP:        &v1alpha1.HTTPConfig{},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "contract-pool-hlb"}, Weight: uint32Ptr(1)},
			},
			DoNotAdvertise: &v1alpha1.EmptyObject{},
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// contractEnsureOriginPool creates a minimal origin pool in the XC API that
// load-balancer contract tests can reference. It registers a cleanup function
// that deletes the pool when the test finishes.
func contractEnsureOriginPool(t *testing.T, xcClient *xcclient.Client, xcNS, name string) {
	t.Helper()
	_ = xcClient.DeleteOriginPool(context.Background(), xcNS, name)
	time.Sleep(time.Second)
	_, err := xcClient.CreateOriginPool(context.Background(), xcNS, &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{Name: name, Namespace: xcNS},
		Spec: xcclient.OriginPoolSpec{
			OriginServers: []xcclient.OriginServer{
				{PublicIP: &xcclient.PublicIP{IP: "198.51.100.1"}},
			},
			Port:  80,
			NoTLS: emptyObjectJSON,
		},
	})
	require.NoError(t, err, "creating dependency origin pool %s", name)
	t.Cleanup(func() {
		_ = xcClient.DeleteOriginPool(context.Background(), xcNS, name)
	})
}

func contractGenerateTestCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "contract-test.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

// ---------------------------------------------------------------------------
// Certificate — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_CertificateCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &CertificateReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-cert"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	_ = xcClient.DeleteCertificate(context.Background(), xcNS, "contract-cert")
	time.Sleep(2 * time.Second)

	certPEM, keyPEM := contractGenerateTestCert(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "contract-cert-tls", Namespace: "contract-cert"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}
	require.NoError(t, testClient.Create(testCtx, secret))

	cr := &v1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-cert",
			Namespace: "contract-cert",
		},
		Spec: v1alpha1.CertificateSpec{
			XCNamespace:         xcNS,
			SecretRef:           v1alpha1.SecretRef{Name: "contract-cert-tls"},
			DisableOcspStapling: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForCertificateConditionResult(t, types.NamespacedName{Name: "contract-cert", Namespace: "contract-cert"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.Contains(t, []string{v1alpha1.ReasonCreateSucceeded, v1alpha1.ReasonUpToDate}, meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionSynced).Reason)
	assert.NotEmpty(t, result.Status.XCUID)

	cert, err := xcClient.GetCertificate(context.Background(), xcNS, "contract-cert")
	require.NoError(t, err)
	assert.NotEmpty(t, cert.Spec.CertificateURL)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetCertificate(context.Background(), xcNS, "contract-cert")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetCertificate(context.Background(), xcNS, "contract-cert")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// OriginPool — field variations
// ---------------------------------------------------------------------------

func TestContract_OriginPoolCRDLifecycle_PublicName(t *testing.T) {
	setupSuite(t)
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	cs := xcclientset.New(xcClient)

	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-op-pn"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteOriginPool(context.Background(), xcNS, "contract-pool-pubname")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-pool-pubname",
			Namespace: "contract-op-pn",
		},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: xcNS,
			OriginServers: []v1alpha1.OriginServer{
				{PublicName: &v1alpha1.PublicName{DNSName: "backend.example.com"}},
			},
			Port: 443,
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "contract-pool-pubname", Namespace: "contract-op-pn"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	assert.NotEmpty(t, result.Status.XCUID)

	pool, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-pubname")
	require.NoError(t, err)
	require.Len(t, pool.Spec.OriginServers, 1)
	require.NotNil(t, pool.Spec.OriginServers[0].PublicName)
	assert.Equal(t, "backend.example.com", pool.Spec.OriginServers[0].PublicName.DNSName)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-pubname")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-pubname")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_OriginPoolCRDLifecycle_MultiOriginWithAlgorithm(t *testing.T) {
	setupSuite(t)
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	cs := xcclientset.New(xcClient)

	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-op-multi"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteOriginPool(context.Background(), xcNS, "contract-pool-multi")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-pool-multi",
			Namespace: "contract-op-multi",
		},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: xcNS,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
				{PublicName: &v1alpha1.PublicName{DNSName: "backend.example.com"}},
				{PublicIP: &v1alpha1.PublicIP{IP: "5.6.7.8"}},
			},
			Port:                  8080,
			LoadBalancerAlgorithm: "ROUND_ROBIN",
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "contract-pool-multi", Namespace: "contract-op-multi"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	assert.NotEmpty(t, result.Status.XCUID)

	pool, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-multi")
	require.NoError(t, err)
	assert.Len(t, pool.Spec.OriginServers, 3)
	assert.Equal(t, 8080, pool.Spec.Port)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-multi")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-multi")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_OriginPoolCRDLifecycle_NoTLS(t *testing.T) {
	setupSuite(t)
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	cs := xcclientset.New(xcClient)

	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-op-notls"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteOriginPool(context.Background(), xcNS, "contract-pool-notls")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-pool-notls",
			Namespace: "contract-op-notls",
		},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: xcNS,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "10.0.0.1"}},
			},
			Port:  80,
			NoTLS: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "contract-pool-notls", Namespace: "contract-op-notls"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	assert.NotEmpty(t, result.Status.XCUID)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-notls")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err := xcClient.GetOriginPool(context.Background(), xcNS, "contract-pool-notls")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// HealthCheck — field variations
// ---------------------------------------------------------------------------

func TestContract_HealthCheckCRDLifecycle_TCP(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-hc-tcp"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteHealthCheck(context.Background(), xcNS, "contract-hc-tcp")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hc-tcp",
			Namespace: "contract-hc-tcp",
		},
		Spec: v1alpha1.HealthCheckSpec{
			XCNamespace: xcNS,
			TCPHealthCheck: &v1alpha1.TCPHealthCheckSpec{
				Send:    "50494e47",
				Receive: "504f4e47",
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForHealthCheckConditionResult(t, types.NamespacedName{Name: "contract-hc-tcp", Namespace: "contract-hc-tcp"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	hc, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-tcp")
	require.NoError(t, err)
	spec, err := hc.ParseSpec()
	require.NoError(t, err)
	require.NotNil(t, spec.TCPHealthCheck)
	assert.Equal(t, "50494e47", spec.TCPHealthCheck.SendPayload)
	assert.Equal(t, "504f4e47", spec.TCPHealthCheck.ExpectedResponse)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-tcp")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-tcp")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

func TestContract_HealthCheckCRDLifecycle_WithThresholds(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-hc-thr"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteHealthCheck(context.Background(), xcNS, "contract-hc-thresholds")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hc-thresholds",
			Namespace: "contract-hc-thr",
		},
		Spec: v1alpha1.HealthCheckSpec{
			XCNamespace:        xcNS,
			HTTPHealthCheck:    &v1alpha1.HTTPHealthCheckSpec{Path: "/ready"},
			HealthyThreshold:   uint32Ptr(5),
			UnhealthyThreshold: uint32Ptr(3),
			Interval:           uint32Ptr(30),
			Timeout:            uint32Ptr(10),
			JitterPercent:      uint32Ptr(20),
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForHealthCheckConditionResult(t, types.NamespacedName{Name: "contract-hc-thresholds", Namespace: "contract-hc-thr"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	hc, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-thresholds")
	require.NoError(t, err)
	spec, err := hc.ParseSpec()
	require.NoError(t, err)
	assert.Equal(t, uint32(5), spec.HealthyThreshold)
	assert.Equal(t, uint32(3), spec.UnhealthyThreshold)
	assert.Equal(t, uint32(30), spec.Interval)
	assert.Equal(t, uint32(10), spec.Timeout)
	assert.Equal(t, uint32(20), spec.JitterPercent)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-thresholds")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetHealthCheck(context.Background(), xcNS, "contract-hc-thresholds")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// AppFirewall — blocking mode variation
// ---------------------------------------------------------------------------

func TestContract_AppFirewallCRDLifecycle_BlockingMode(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-afw-blk"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteAppFirewall(context.Background(), xcNS, "contract-afw-blocking")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-afw-blocking",
			Namespace: "contract-afw-blk",
		},
		Spec: v1alpha1.AppFirewallSpec{
			XCNamespace:          xcNS,
			Blocking:             &v1alpha1.EmptyObject{},
			DisableAnonymization: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForAppFirewallConditionResult(t, types.NamespacedName{Name: "contract-afw-blocking", Namespace: "contract-afw-blk"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw-blocking")
	require.NoError(t, err)
	assert.NotNil(t, obj.Spec.Blocking)
	assert.NotNil(t, obj.Spec.DisableAnonymization)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw-blocking")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetAppFirewall(context.Background(), xcNS, "contract-afw-blocking")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// RateLimiter — with burst multiplier
// ---------------------------------------------------------------------------

func TestContract_RateLimiterCRDLifecycle_WithBurst(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-rl-burst"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteRateLimiter(context.Background(), xcNS, "contract-rl-burst")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.RateLimiter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-rl-burst",
			Namespace: "contract-rl-burst",
		},
		Spec: v1alpha1.RateLimiterSpec{
			XCNamespace:     xcNS,
			Threshold:       50,
			Unit:            "SECOND",
			BurstMultiplier: uint32Ptr(3),
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForRateLimiterConditionResult(t, types.NamespacedName{Name: "contract-rl-burst", Namespace: "contract-rl-burst"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	rl, err := xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl-burst")
	require.NoError(t, err)
	require.Len(t, rl.Spec.Limits, 1)
	assert.Equal(t, uint32(50), rl.Spec.Limits[0].TotalNumber)
	assert.Equal(t, uint32(3), rl.Spec.Limits[0].BurstMultiplier)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl-burst")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetRateLimiter(context.Background(), xcNS, "contract-rl-burst")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// ServicePolicy — deny_all with serverName
// ---------------------------------------------------------------------------

func TestContract_ServicePolicyCRDLifecycle_DenyAll(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-sp-deny"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteServicePolicy(context.Background(), xcNS, "contract-sp-deny")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.ServicePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-sp-deny",
			Namespace: "contract-sp-deny",
		},
		Spec: v1alpha1.ServicePolicySpec{
			XCNamespace:     xcNS,
			DenyAllRequests: &v1alpha1.EmptyObject{},
			ServerName:      "api.example.com",
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForServicePolicyConditionResult(t, types.NamespacedName{Name: "contract-sp-deny", Namespace: "contract-sp-deny"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	sp, err := xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp-deny")
	require.NoError(t, err)
	assert.NotNil(t, sp.Spec.DenyAllRequests)
	assert.Equal(t, "api.example.com", sp.Spec.ServerName)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp-deny")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetServicePolicy(context.Background(), xcNS, "contract-sp-deny")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// HTTPLoadBalancer — HTTPS auto-cert variation
// ---------------------------------------------------------------------------

func TestContract_HTTPLoadBalancerCRDLifecycle_HTTPSAutoCert(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-hlb-ac"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	contractEnsureOriginPool(t, xcClient, xcNS, "contract-pool-hlb-ac")

	_ = xcClient.DeleteHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb-autocert")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.HTTPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-hlb-autocert",
			Namespace: "contract-hlb-ac",
		},
		Spec: v1alpha1.HTTPLoadBalancerSpec{
			XCNamespace: xcNS,
			Domains:     []string{"autocert.contract.example.com"},
			HTTPSAutoCert: &v1alpha1.HTTPSAutoCertConfig{AddHSTS: true, HTTPRedirect: true, NoMTLS: &v1alpha1.EmptyObject{}},
			DefaultRoutePools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "contract-pool-hlb-ac"}, Weight: uint32Ptr(1)},
			},
			DoNotAdvertise: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForHTTPLoadBalancerConditionResult(t, types.NamespacedName{Name: "contract-hlb-autocert", Namespace: "contract-hlb-ac"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	hlb, err := xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb-autocert")
	require.NoError(t, err)
	assert.NotNil(t, hlb.Spec.HTTPSAutoCert)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb-autocert")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetHTTPLoadBalancer(context.Background(), xcNS, "contract-hlb-autocert")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// TCPLoadBalancer — doNotAdvertise variation
// ---------------------------------------------------------------------------

func TestContract_TCPLoadBalancerCRDLifecycle_DoNotAdvertise(t *testing.T) {
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

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-tlb-noadv"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	contractEnsureOriginPool(t, xcClient, xcNS, "contract-pool-tlb-noadv")

	_ = xcClient.DeleteTCPLoadBalancer(context.Background(), xcNS, "contract-tlb-noadv")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.TCPLoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-tlb-noadv",
			Namespace: "contract-tlb-noadv",
		},
		Spec: v1alpha1.TCPLoadBalancerSpec{
			XCNamespace: xcNS,
			Domains:     []string{"contract-tlb-noadv.k8s-op-test.example.com"},
			ListenPort:  8080,
			OriginPools: []v1alpha1.RoutePool{
				{Pool: v1alpha1.ObjectRef{Name: "contract-pool-tlb-noadv"}, Weight: uint32Ptr(1)},
			},
			DoNotAdvertise: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForTCPLoadBalancerConditionResult(t, types.NamespacedName{Name: "contract-tlb-noadv", Namespace: "contract-tlb-noadv"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	tlb, err := xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb-noadv")
	require.NoError(t, err)
	assert.NotNil(t, tlb.Spec.DoNotAdvertise)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb-noadv")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetTCPLoadBalancer(context.Background(), xcNS, "contract-tlb-noadv")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// APIDefinition — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_APIDefinitionCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &APIDefinitionReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-apidef"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteAPIDefinition(context.Background(), xcNS, "contract-apidef")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.APIDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-apidef",
			Namespace: "contract-apidef",
		},
		Spec: v1alpha1.APIDefinitionSpec{
			XCNamespace:       xcNS,
			MixedSchemaOrigin: &v1alpha1.EmptyObject{},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForAPIDefinitionConditionResult(t, types.NamespacedName{Name: "contract-apidef", Namespace: "contract-apidef"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetAPIDefinition(context.Background(), xcNS, "contract-apidef")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// UserIdentification — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_UserIdentificationCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &UserIdentificationReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-uid"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteUserIdentification(context.Background(), xcNS, "contract-uid")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.UserIdentification{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-uid",
			Namespace: "contract-uid",
		},
		Spec: v1alpha1.UserIdentificationSpec{
			XCNamespace: xcNS,
			Rules: []v1alpha1.UserIdentificationRule{
				{ClientIP: &v1alpha1.EmptyObject{}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForUserIdentificationConditionResult(t, types.NamespacedName{Name: "contract-uid", Namespace: "contract-uid"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetUserIdentification(context.Background(), xcNS, "contract-uid")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}

// ---------------------------------------------------------------------------
// MaliciousUserMitigation — full lifecycle
// ---------------------------------------------------------------------------

func TestContract_MaliciousUserMitigationCRDLifecycle(t *testing.T) {
	xcClient := contractXCClient(t)
	xcNS := contractNamespace(t)
	setupSuite(t)
	cs := xcclientset.New(xcClient)

	reconciler := &MaliciousUserMitigationReconciler{
		Log:       ctrl.Log.WithName("contract"),
		ClientSet: cs,
	}
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-mum"}}
	require.NoError(t, testClient.Create(testCtx, ns))
	_ = xcClient.DeleteMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	time.Sleep(2 * time.Second)

	cr := &v1alpha1.MaliciousUserMitigation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contract-mum",
			Namespace: "contract-mum",
		},
		Spec: v1alpha1.MaliciousUserMitigationSpec{
			XCNamespace: xcNS,
			MitigationType: &v1alpha1.MaliciousUserMitigationType{
				Rules: []v1alpha1.MaliciousUserMitigationRule{
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Low: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{JavascriptChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{Medium: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{CaptchaChallenge: &v1alpha1.EmptyObject{}},
					},
					{
						ThreatLevel:      v1alpha1.MaliciousUserThreatLevel{High: &v1alpha1.EmptyObject{}},
						MitigationAction: v1alpha1.MaliciousUserMitigationAction{BlockTemporarily: &v1alpha1.EmptyObject{}},
					},
				},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForMaliciousUserMitigationConditionResult(t, types.NamespacedName{Name: "contract-mum", Namespace: "contract-mum"}, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Status.XCUID)

	obj, err := xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	require.NoError(t, err)
	assert.NotNil(t, obj)

	require.NoError(t, testClient.Delete(testCtx, cr))
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_, err := xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
		if err != nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	_, err = xcClient.GetMaliciousUserMitigation(context.Background(), xcNS, "contract-mum")
	assert.ErrorIs(t, err, xcclient.ErrNotFound)
}
