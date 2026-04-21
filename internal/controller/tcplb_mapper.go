package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildTCPLoadBalancerCreate(cr *v1alpha1.TCPLoadBalancer, xcNamespace string) *xcclient.TCPLoadBalancerCreate {
	return &xcclient.TCPLoadBalancerCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapTCPLoadBalancerSpec(&cr.Spec),
	}
}

func buildTCPLoadBalancerReplace(cr *v1alpha1.TCPLoadBalancer, xcNamespace, resourceVersion string) *xcclient.TCPLoadBalancerReplace {
	return &xcclient.TCPLoadBalancerReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapTCPLoadBalancerSpec(&cr.Spec),
	}
}

func buildTCPLoadBalancerDesiredSpecJSON(cr *v1alpha1.TCPLoadBalancer, xcNamespace string) (json.RawMessage, error) {
	create := buildTCPLoadBalancerCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapTCPLoadBalancerSpec(spec *v1alpha1.TCPLoadBalancerSpec) xcclient.TCPLoadBalancerSpec {
	out := xcclient.TCPLoadBalancerSpec{
		Domains:    spec.Domains,
		ListenPort: spec.ListenPort,
	}

	for i := range spec.OriginPools {
		out.OriginPools = append(out.OriginPools, mapRoutePool(&spec.OriginPools[i]))
	}

	// TLS OneOf
	if spec.NoTLS != nil {
		out.NoTLS = json.RawMessage(spec.NoTLS.Raw)
	}
	if spec.TLSParameters != nil {
		out.TLSParameters = json.RawMessage(spec.TLSParameters.Raw)
	}
	if spec.TLSPassthrough != nil {
		out.TLSPassthrough = json.RawMessage(spec.TLSPassthrough.Raw)
	}

	// Advertise OneOf
	if spec.AdvertiseOnPublicDefaultVIP != nil {
		out.AdvertiseOnPublicDefaultVIP = json.RawMessage(spec.AdvertiseOnPublicDefaultVIP.Raw)
	}
	if spec.AdvertiseOnPublic != nil {
		out.AdvertiseOnPublic = json.RawMessage(spec.AdvertiseOnPublic.Raw)
	}
	if spec.AdvertiseCustom != nil {
		out.AdvertiseCustom = json.RawMessage(spec.AdvertiseCustom.Raw)
	}
	if spec.DoNotAdvertise != nil {
		out.DoNotAdvertise = json.RawMessage(spec.DoNotAdvertise.Raw)
	}

	return out
}

func mapRoutePool(rp *v1alpha1.RoutePool) xcclient.RoutePool {
	out := xcclient.RoutePool{
		Pool: mapObjectRef(&rp.Pool),
	}
	if rp.Weight != nil {
		out.Weight = *rp.Weight
	}
	if rp.Priority != nil {
		out.Priority = *rp.Priority
	}
	return out
}
