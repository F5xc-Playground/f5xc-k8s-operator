package controller

import (
	"encoding/json"
	"testing"
	"time"

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

func TestIntegrationDiscover_CreateWithServiceLB(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-svc", Namespace: "intd-create"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool", Namespace: "intd-create"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "intd-pool", Namespace: "intd-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NotNil(t, result)

	// Verify the XC server received a POST with publicIP
	requests := srv.Requests()
	var postFound bool
	for _, r := range requests {
		if r.Method == "POST" {
			postFound = true
			var body struct {
				Spec struct {
					OriginServers []struct {
						PublicIP *struct {
							IP string `json:"ip"`
						} `json:"public_ip,omitempty"`
					} `json:"origin_servers"`
				} `json:"spec"`
			}
			require.NoError(t, json.Unmarshal(r.Body, &body))
			require.Len(t, body.Spec.OriginServers, 1)
			require.NotNil(t, body.Spec.OriginServers[0].PublicIP)
			assert.Equal(t, "198.51.100.1", body.Spec.OriginServers[0].PublicIP.IP)
		}
	}
	assert.True(t, postFound)
}

func TestIntegrationDiscover_PendingThenResolved(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-pending"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pending-svc", Namespace: "intd-pending"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-pending", Namespace: "intd-pending"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-pending-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "intd-pool-pending", Namespace: "intd-pending"}
	result := waitForConditionResult(t, key, v1alpha1.ConditionReady, metav1.ConditionFalse, 15*time.Second)
	require.NotNil(t, result)
	cond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)

	// Now assign IP to Service
	require.NoError(t, testClient.Get(testCtx, types.NamespacedName{Name: "intd-pending-svc", Namespace: "intd-pending"}, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.2"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	// Should eventually become Ready
	result = waitForConditionResult(t, key, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
}

func TestIntegrationDiscover_ServiceIPChanges(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-change"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-change-svc", Namespace: "intd-change"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.10"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-change", Namespace: "intd-change"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-change-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "intd-pool-change", Namespace: "intd-change"}
	waitForConditionResult(t, key, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)

	// Clear requests and change IP
	srv.ClearRequests()
	require.NoError(t, testClient.Get(testCtx, types.NamespacedName{Name: "intd-change-svc", Namespace: "intd-change"}, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.20"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	// Wait for a PUT with the new IP
	deadline := time.Now().Add(30 * time.Second)
	var putFound bool
	for time.Now().Before(deadline) {
		for _, r := range srv.Requests() {
			if r.Method == "PUT" {
				var body struct {
					Spec struct {
						OriginServers []struct {
							PublicIP *struct {
								IP string `json:"ip"`
							} `json:"public_ip,omitempty"`
						} `json:"origin_servers"`
					} `json:"spec"`
				}
				if err := json.Unmarshal(r.Body, &body); err == nil &&
					len(body.Spec.OriginServers) > 0 &&
					body.Spec.OriginServers[0].PublicIP != nil &&
					body.Spec.OriginServers[0].PublicIP.IP == "198.51.100.20" {
					putFound = true
				}
			}
		}
		if putFound {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	assert.True(t, putFound, "expected a PUT with the updated IP 198.51.100.20")
}

func TestIntegrationDiscover_DeleteWithDiscoverOrigin(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-del-svc", Namespace: "intd-delete"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.30"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-del", Namespace: "intd-delete"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-del-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "intd-pool-del", Namespace: "intd-delete"}
	waitForConditionResult(t, key, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)

	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.OriginPool
		err := testClient.Get(testCtx, key, &check)
		if err != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	var deleteFound bool
	for _, r := range srv.Requests() {
		if r.Method == "DELETE" {
			deleteFound = true
		}
	}
	assert.True(t, deleteFound, "expected a DELETE request to the fake server")
}
