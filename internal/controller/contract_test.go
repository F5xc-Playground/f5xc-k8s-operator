//go:build contract

package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/go-logr/logr/funcr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
