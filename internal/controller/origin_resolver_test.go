package controller

import (
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	routev1 "github.com/openshift/api/route/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestResolveService_LoadBalancerWithIP(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.50"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.50", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_LoadBalancerWithHostname(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 80}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "a1b2c3.elb.amazonaws.com"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "a1b2c3.elb.amazonaws.com", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_LoadBalancerPending(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{},
	}

	result := ResolveService(svc, nil)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no loadBalancer ingress")
}

func TestResolveService_NodePort(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{NodePort: 30080}},
		},
	}
	nodes := []corev1.Node{
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeExternalIP, Address: "198.51.100.10"},
				},
			},
		},
	}

	result := ResolveService(svc, nodes)
	assert.False(t, result.Pending)
	assert.Equal(t, "198.51.100.10", result.Address)
	assert.Equal(t, uint32(30080), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_NodePortNoExternalIP(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{NodePort: 30080}},
		},
	}
	nodes := []corev1.Node{
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		},
	}

	result := ResolveService(svc, nodes)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no nodes with external IP")
}

func TestResolveService_ExternalName(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "backend.example.com",
			Ports:        []corev1.ServicePort{{Port: 8443}},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "backend.example.com", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_ExternalNameNoPort(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "backend.example.com",
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "backend.example.com", result.Address)
	assert.Equal(t, uint32(0), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_ExternalIPs(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceTypeClusterIP,
			ExternalIPs: []string{"203.0.113.100"},
			Ports:       []corev1.ServicePort{{Port: 443}},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.100", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_ClusterIPOnly(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}

	result := ResolveService(svc, nil)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "ClusterIP")
}

func TestResolveService_ExternalIPsTakesPriorityOverLoadBalancer(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceTypeLoadBalancer,
			ExternalIPs: []string{"198.51.100.50"},
			Ports:       []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.1"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.Equal(t, "198.51.100.50", result.Address)
}

func TestResolveDiscover_AddressOverride(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "10.0.0.1",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "203.0.113.50",
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "203.0.113.50", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveDiscover_PortOverride(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "1.2.3.4",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	port := uint32(8443)
	discover := &v1alpha1.OriginServerDiscover{
		PortOverride: &port,
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "1.2.3.4", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
}

func TestResolveDiscover_BothOverrides(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "10.0.0.1",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	port := uint32(443)
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "lb.example.com",
		PortOverride:    &port,
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "lb.example.com", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveDiscover_AddressOverrideIP(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "host.example.com",
		Port:        443,
		AddressType: v1alpha1.AddressTypeFQDN,
	}
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "198.51.100.1",
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveIngress_WithIP(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "203.0.113.10"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.10", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveIngress_WithHostname(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.com"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.False(t, result.Pending)
	assert.Equal(t, "ingress.example.com", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveIngress_WithTLS(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.Equal(t, uint32(443), result.Port)
}

func TestResolveIngress_NoTLS(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.Equal(t, uint32(80), result.Port)
}

func TestResolveIngress_Pending(t *testing.T) {
	ing := &networkingv1.Ingress{}

	result := ResolveIngress(ing)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no loadBalancer ingress")
}

func TestResolveGateway_WithIPAddress(t *testing.T) {
	addrType := gatewayv1.IPAddressType
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Name: "default", Port: 8443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Type: &addrType, Value: "203.0.113.20"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.20", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveGateway_WithHostname(t *testing.T) {
	addrType := gatewayv1.HostnameAddressType
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Name: "default", Port: 443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Type: &addrType, Value: "gateway.example.com"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "gateway.example.com", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveGateway_Pending(t *testing.T) {
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Name: "default", Port: 443}},
		},
	}

	result := ResolveGateway(gw)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no addresses")
}

func TestResolveGateway_NilAddressType(t *testing.T) {
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Name: "default", Port: 443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Value: "1.2.3.4"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "1.2.3.4", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveRoute_AdmittedWithTLS(t *testing.T) {
	route := &routev1.Route{
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge},
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.False(t, result.Pending)
	assert.Equal(t, "myapp.apps.example.com", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveRoute_AdmittedNoTLS(t *testing.T) {
	route := &routev1.Route{
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.False(t, result.Pending)
	assert.Equal(t, uint32(80), result.Port)
}

func TestResolveRoute_NotAdmitted(t *testing.T) {
	route := &routev1.Route{
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionFalse},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "not admitted")
}

func TestResolveRoute_NoIngress(t *testing.T) {
	route := &routev1.Route{}

	result := ResolveRoute(route)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no ingress status")
}

func TestResolveRoute_MultiIngressSecondAdmitted(t *testing.T) {
	route := &routev1.Route{
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "a.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionFalse},
					},
				},
				{
					Host: "b.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.False(t, result.Pending)
	assert.Equal(t, "b.example.com", result.Address)
	assert.Equal(t, uint32(80), result.Port)
}
