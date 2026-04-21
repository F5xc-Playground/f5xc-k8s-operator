package controller

import (
	"fmt"
	"net"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type ResolvedOrigin struct {
	Address     string
	Port        uint32
	AddressType string
	Pending     bool
	Message     string
}

func classifyAddress(addr string) string {
	if net.ParseIP(addr) != nil {
		return v1alpha1.AddressTypeIP
	}
	return v1alpha1.AddressTypeFQDN
}

func ResolveService(svc *corev1.Service, nodes []corev1.Node) ResolvedOrigin {
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return resolveExternalName(svc)
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return resolveExternalIPs(svc)
	}
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		return resolveLoadBalancer(svc)
	}
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		return resolveNodePort(svc, nodes)
	}
	return ResolvedOrigin{
		Pending: true,
		Message: "Service type ClusterIP is not externally routable",
	}
}

func resolveLoadBalancer(svc *corev1.Service) ResolvedOrigin {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Service has no loadBalancer ingress assigned",
		}
	}
	ingress := svc.Status.LoadBalancer.Ingress[0]
	addr := ingress.IP
	if addr == "" {
		addr = ingress.Hostname
	}
	return ResolvedOrigin{
		Address:     addr,
		Port:        servicePort(svc),
		AddressType: classifyAddress(addr),
	}
}

func resolveNodePort(svc *corev1.Service, nodes []corev1.Node) ResolvedOrigin {
	for _, node := range nodes {
		if !isNodeReady(&node) {
			continue
		}
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				var port uint32
				if len(svc.Spec.Ports) > 0 {
					port = uint32(svc.Spec.Ports[0].NodePort)
				}
				return ResolvedOrigin{
					Address:     addr.Address,
					Port:        port,
					AddressType: v1alpha1.AddressTypeIP,
				}
			}
		}
	}
	return ResolvedOrigin{
		Pending: true,
		Message: "Service type NodePort but no nodes with external IP found",
	}
}

func resolveExternalName(svc *corev1.Service) ResolvedOrigin {
	return ResolvedOrigin{
		Address:     svc.Spec.ExternalName,
		Port:        servicePort(svc),
		AddressType: v1alpha1.AddressTypeFQDN,
	}
}

func resolveExternalIPs(svc *corev1.Service) ResolvedOrigin {
	return ResolvedOrigin{
		Address:     svc.Spec.ExternalIPs[0],
		Port:        servicePort(svc),
		AddressType: classifyAddress(svc.Spec.ExternalIPs[0]),
	}
}

func servicePort(svc *corev1.Service) uint32 {
	if len(svc.Spec.Ports) > 0 {
		return uint32(svc.Spec.Ports[0].Port)
	}
	return 0
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func ResolveIngress(ing *networkingv1.Ingress) ResolvedOrigin {
	if len(ing.Status.LoadBalancer.Ingress) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Ingress has no loadBalancer ingress assigned",
		}
	}

	ingress := ing.Status.LoadBalancer.Ingress[0]
	addr := ingress.IP
	if addr == "" {
		addr = ingress.Hostname
	}

	port := uint32(80)
	if len(ing.Spec.TLS) > 0 {
		port = 443
	}

	return ResolvedOrigin{
		Address:     addr,
		Port:        port,
		AddressType: classifyAddress(addr),
	}
}

func ResolveDiscover(discover *v1alpha1.OriginServerDiscover, resolved ResolvedOrigin) ResolvedOrigin {
	if discover.AddressOverride != "" {
		resolved.Address = discover.AddressOverride
		resolved.AddressType = classifyAddress(discover.AddressOverride)
	}
	if discover.PortOverride != nil {
		resolved.Port = *discover.PortOverride
	}
	return resolved
}

func UnsupportedKindError(kind string) ResolvedOrigin {
	return ResolvedOrigin{
		Pending: true,
		Message: fmt.Sprintf("unsupported resource kind %q", kind),
	}
}
