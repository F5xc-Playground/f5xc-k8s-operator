package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcTLSParameters struct {
	TLSCertificates []xcTLSCertificateRef `json:"tls_certificates,omitempty"`
	DefaultSecurity *v1alpha1.EmptyObject `json:"default_security,omitempty"`
	LowSecurity     *v1alpha1.EmptyObject `json:"low_security,omitempty"`
	MediumSecurity  *v1alpha1.EmptyObject `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity  `json:"custom_security,omitempty"`
	NoMTLS          *v1alpha1.EmptyObject `json:"no_mtls,omitempty"`
	UseMTLS         *xcUseMTLS            `json:"use_mtls,omitempty"`
}

type xcTLSTCPAutoCert struct {
	NoMTLS  *v1alpha1.EmptyObject `json:"no_mtls,omitempty"`
	UseMTLS *xcUseMTLS            `json:"use_mtls,omitempty"`
}

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
		out.OriginPoolWeights = append(out.OriginPoolWeights, mapRoutePool(&spec.OriginPools[i]))
	}

	// TLS OneOf
	if spec.NoTLS != nil {
		out.TCP = emptyObjectJSON
	}
	if spec.TLSParameters != nil {
		out.TLSTCP = mapTLSParameters(spec.TLSParameters)
	}
	if spec.TLSTCPAutoCert != nil {
		out.TLSTCPAutoCert = marshalJSON(xcTLSTCPAutoCert{
			NoMTLS:  spec.TLSTCPAutoCert.NoMTLS,
			UseMTLS: mapXCUseMTLS(spec.TLSTCPAutoCert.UseMTLS),
		})
	}

	// Advertise OneOf
	if spec.AdvertiseOnPublicDefaultVIP != nil {
		out.AdvertiseOnPublicDefaultVIP = emptyObjectJSON
	}
	if spec.AdvertiseOnPublic != nil {
		out.AdvertiseOnPublic = mapXCAdvertiseOnPublic(spec.AdvertiseOnPublic)
	}
	if spec.AdvertiseCustom != nil {
		out.AdvertiseCustom = mapXCAdvertiseCustom(spec.AdvertiseCustom)
	}
	if spec.DoNotAdvertise != nil {
		out.DoNotAdvertise = emptyObjectJSON
	}

	return out
}

func mapTLSParameters(p *v1alpha1.TLSParameters) json.RawMessage {
	return marshalJSON(xcTLSParameters{
		TLSCertificates: mapXCTLSCertificateRefs(p.TLSCertificates),
		DefaultSecurity: p.DefaultSecurity,
		LowSecurity:     p.LowSecurity,
		MediumSecurity:  p.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(p.CustomSecurity),
		NoMTLS:          p.NoMTLS,
		UseMTLS:         mapXCUseMTLS(p.UseMTLS),
	})
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
