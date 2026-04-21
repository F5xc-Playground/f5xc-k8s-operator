package controller

import (
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDiscover_ServiceLoadBalancerIP(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-svc-ip"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "disc-svc-ip"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "203.0.113.50"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-ip", Namespace: "disc-svc-ip"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-svc-ip",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "my-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-ip", Namespace: "disc-svc-ip"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.True(t, created, "expected CreateOriginPool to be called")

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, v1alpha1.DiscoveryStatusResolved, updated.Status.DiscoveredOrigins[0].Status)
	assert.Equal(t, "203.0.113.50", updated.Status.DiscoveredOrigins[0].Address)
	assert.Equal(t, v1alpha1.AddressTypeIP, updated.Status.DiscoveredOrigins[0].AddressType)
}

func TestDiscover_IngressHostname(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-ing"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ing", Namespace: "disc-ing"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "ingress.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "dummy",
											Port: networkingv1.ServiceBackendPort{Number: 443},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, ing))

	ing.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.com"}}
	require.NoError(t, testClient.Status().Update(testCtx, ing))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-ing", Namespace: "disc-ing"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-ing",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Ingress", Name: "my-ing"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-ing", Namespace: "disc-ing"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, v1alpha1.DiscoveryStatusResolved, updated.Status.DiscoveredOrigins[0].Status)
	assert.Equal(t, "ingress.example.com", updated.Status.DiscoveredOrigins[0].Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, updated.Status.DiscoveredOrigins[0].AddressType)
}

func TestDiscover_Pending(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-pending"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-svc", Namespace: "disc-pending"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-pending", Namespace: "disc-pending"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-pending",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "pending-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-pending", Namespace: "disc-pending"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.False(t, created, "expected CreateOriginPool NOT to be called when pending")
}

func TestDiscover_MixedStaticAndDiscover(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-mixed"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed-svc", Namespace: "disc-mixed"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-mixed", Namespace: "disc-mixed"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-mixed",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "mixed-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-mixed", Namespace: "disc-mixed"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.True(t, created, "expected create with both static and discovered origins")
}

func TestDiscover_AddressOverride(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-override"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "override-svc", Namespace: "disc-override"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-override", Namespace: "disc-override"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-override",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource:        v1alpha1.ResourceRef{Kind: "Service", Name: "override-svc"},
					AddressOverride: "203.0.113.99",
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-override", Namespace: "disc-override"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, "203.0.113.99", updated.Status.DiscoveredOrigins[0].Address)
}

func TestDiscover_MissingResource(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-missing"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-missing", Namespace: "disc-missing"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-missing",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "nonexistent"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-missing", Namespace: "disc-missing"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)
}

func TestDiscover_CrossNamespace(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	nsA := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-xns-a"}}
	nsB := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-xns-b"}}
	require.NoError(t, testClient.Create(testCtx, nsA))
	require.NoError(t, testClient.Create(testCtx, nsB))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "xns-svc", Namespace: "disc-xns-b"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-xns", Namespace: "disc-xns-a"},
		Spec: v1alpha1.OriginPoolSpec{
			XCNamespace: "disc-xns-a",
			Port:        443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "xns-svc", Namespace: "disc-xns-b"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-xns", Namespace: "disc-xns-a"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))
	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, "198.51.100.1", updated.Status.DiscoveredOrigins[0].Address)
}
