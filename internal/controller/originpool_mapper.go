package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildOriginPoolCreate(cr *v1alpha1.OriginPool, xcNamespace string) *xcclient.OriginPoolCreate {
	return &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapOriginPoolSpec(&cr.Spec),
	}
}

func buildOriginPoolReplace(cr *v1alpha1.OriginPool, xcNamespace, resourceVersion string) *xcclient.OriginPoolReplace {
	return &xcclient.OriginPoolReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapOriginPoolSpec(&cr.Spec),
	}
}

func buildOriginPoolDesiredSpecJSON(cr *v1alpha1.OriginPool, xcNamespace string) (json.RawMessage, error) {
	create := buildOriginPoolCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapOriginPoolSpec(spec *v1alpha1.OriginPoolSpec) xcclient.OriginPoolSpec {
	out := xcclient.OriginPoolSpec{
		Port:                  spec.Port,
		LoadBalancerAlgorithm: spec.LoadBalancerAlgorithm,
	}

	for _, s := range spec.OriginServers {
		out.OriginServers = append(out.OriginServers, mapOriginServer(&s))
	}

	for _, hc := range spec.HealthChecks {
		out.HealthCheck = append(out.HealthCheck, mapObjectRef(&hc))
	}

	if spec.UseTLS != nil {
		out.UseTLS = json.RawMessage(spec.UseTLS.Raw)
	}
	if spec.NoTLS != nil {
		out.NoTLS = json.RawMessage(spec.NoTLS.Raw)
	}

	return out
}

func mapOriginServer(s *v1alpha1.OriginServer) xcclient.OriginServer {
	var out xcclient.OriginServer

	if s.PublicIP != nil {
		out.PublicIP = &xcclient.PublicIP{IP: s.PublicIP.IP}
	}
	if s.PublicName != nil {
		out.PublicName = &xcclient.PublicName{DNSName: s.PublicName.DNSName}
	}
	if s.PrivateIP != nil {
		out.PrivateIP = &xcclient.PrivateIP{
			IP:   s.PrivateIP.IP,
			Site: mapObjectRefPtr(s.PrivateIP.Site),
		}
	}
	if s.PrivateName != nil {
		out.PrivateName = &xcclient.PrivateName{
			DNSName: s.PrivateName.DNSName,
			Site:    mapObjectRefPtr(s.PrivateName.Site),
		}
	}
	if s.K8SService != nil {
		out.K8SService = &xcclient.K8SService{
			ServiceName:      s.K8SService.ServiceName,
			ServiceNamespace: s.K8SService.ServiceNamespace,
			Site:             mapObjectRefPtr(s.K8SService.Site),
		}
	}
	if s.ConsulService != nil {
		out.ConsulService = &xcclient.ConsulService{
			ServiceName: s.ConsulService.ServiceName,
			Site:        mapObjectRefPtr(s.ConsulService.Site),
		}
	}

	return out
}

func mapObjectRef(ref *v1alpha1.ObjectRef) xcclient.ObjectRef {
	return xcclient.ObjectRef{
		Name:      ref.Name,
		Namespace: ref.Namespace,
		Tenant:    ref.Tenant,
	}
}

func mapObjectRefPtr(ref *v1alpha1.ObjectRef) *xcclient.ObjectRef {
	if ref == nil {
		return nil
	}
	out := mapObjectRef(ref)
	return &out
}
