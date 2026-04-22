package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

// Wire-format types for OriginPool TLS (snake_case JSON tags — mirrors F5XC REST API)

type xcOriginPoolTLS struct {
	TLSConfig              *xcTLSConfig         `json:"tls_config,omitempty"`
	SNI                    string               `json:"sni,omitempty"`
	VolterraTrustedCA      *v1alpha1.EmptyObject `json:"volterra_trusted_ca,omitempty"`
	TrustedCAURL           string               `json:"trusted_ca_url,omitempty"`
	DisableSNI             *v1alpha1.EmptyObject `json:"disable_sni,omitempty"`
	UseServerVerification  *v1alpha1.EmptyObject `json:"use_server_verification,omitempty"`
	SkipServerVerification *v1alpha1.EmptyObject `json:"skip_server_verification,omitempty"`
	NoMTLS                 *v1alpha1.EmptyObject `json:"no_mtls,omitempty"`
}

type xcTLSConfig struct {
	DefaultSecurity *v1alpha1.EmptyObject `json:"default_security,omitempty"`
	LowSecurity     *v1alpha1.EmptyObject `json:"low_security,omitempty"`
	MediumSecurity  *v1alpha1.EmptyObject `json:"medium_security,omitempty"`
	CustomSecurity  *xcCustomTLSSecurity  `json:"custom_security,omitempty"`
}

func buildOriginPoolCreate(cr *v1alpha1.OriginPool, xcNamespace string, resolved []*ResolvedOrigin) *xcclient.OriginPoolCreate {
	return &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapOriginPoolSpec(&cr.Spec, resolved),
	}
}

func buildOriginPoolReplace(cr *v1alpha1.OriginPool, xcNamespace, resourceVersion string, resolved []*ResolvedOrigin) *xcclient.OriginPoolReplace {
	return &xcclient.OriginPoolReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapOriginPoolSpec(&cr.Spec, resolved),
	}
}

func buildOriginPoolDesiredSpecJSON(cr *v1alpha1.OriginPool, xcNamespace string, resolved []*ResolvedOrigin) (json.RawMessage, error) {
	create := buildOriginPoolCreate(cr, xcNamespace, resolved)
	return json.Marshal(create.Spec)
}

func mapOriginPoolSpec(spec *v1alpha1.OriginPoolSpec, resolved []*ResolvedOrigin) xcclient.OriginPoolSpec {
	out := xcclient.OriginPoolSpec{
		Port:                  spec.Port,
		LoadBalancerAlgorithm: spec.LoadBalancerAlgorithm,
	}

	for i, s := range spec.OriginServers {
		if resolved != nil && i < len(resolved) && resolved[i] != nil {
			out.OriginServers = append(out.OriginServers, mapResolvedOriginServer(resolved[i]))
		} else {
			out.OriginServers = append(out.OriginServers, mapOriginServer(&s))
		}
	}

	for _, hc := range spec.HealthChecks {
		out.HealthCheck = append(out.HealthCheck, mapObjectRef(&hc))
	}

	if spec.UseTLS != nil {
		out.UseTLS = mapOriginPoolTLS(spec.UseTLS)
	}
	if spec.NoTLS != nil {
		out.NoTLS = emptyObjectJSON
	}

	return out
}

func mapResolvedOriginServer(r *ResolvedOrigin) xcclient.OriginServer {
	var out xcclient.OriginServer
	if r.AddressType == v1alpha1.AddressTypeIP {
		out.PublicIP = &xcclient.PublicIP{IP: r.Address}
	} else {
		out.PublicName = &xcclient.PublicName{DNSName: r.Address}
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
		p := &xcclient.PrivateIP{IP: s.PrivateIP.IP}
		p.SiteLocator = mapSiteLocator(s.PrivateIP.Site, s.PrivateIP.VirtualSite)
		p.InsideNetwork, p.OutsideNetwork = mapNetworkChoice(s.PrivateIP.InsideNetwork, s.PrivateIP.OutsideNetwork)
		out.PrivateIP = p
	}
	if s.PrivateName != nil {
		p := &xcclient.PrivateName{DNSName: s.PrivateName.DNSName}
		p.SiteLocator = mapSiteLocator(s.PrivateName.Site, s.PrivateName.VirtualSite)
		p.InsideNetwork, p.OutsideNetwork = mapNetworkChoice(s.PrivateName.InsideNetwork, s.PrivateName.OutsideNetwork)
		out.PrivateName = p
	}
	if s.K8SService != nil {
		k := &xcclient.K8SService{
			ServiceName:      s.K8SService.ServiceName,
			ServiceNamespace: s.K8SService.ServiceNamespace,
		}
		k.SiteLocator = mapSiteLocator(s.K8SService.Site, s.K8SService.VirtualSite)
		k.InsideNetwork, k.OutsideNetwork = mapNetworkChoice(s.K8SService.InsideNetwork, s.K8SService.OutsideNetwork)
		out.K8SService = k
	}
	if s.ConsulService != nil {
		c := &xcclient.ConsulService{ServiceName: s.ConsulService.ServiceName}
		c.SiteLocator = mapSiteLocator(s.ConsulService.Site, s.ConsulService.VirtualSite)
		c.InsideNetwork, c.OutsideNetwork = mapNetworkChoice(s.ConsulService.InsideNetwork, s.ConsulService.OutsideNetwork)
		out.ConsulService = c
	}

	return out
}

func mapSiteLocator(site, virtualSite *v1alpha1.ObjectRef) *xcclient.SiteLocator {
	if site == nil && virtualSite == nil {
		return nil
	}
	sl := &xcclient.SiteLocator{}
	if site != nil {
		sl.Site = mapObjectRefPtr(site)
	}
	if virtualSite != nil {
		sl.VirtualSite = mapObjectRefPtr(virtualSite)
	}
	return sl
}

func mapOriginPoolTLS(tls *v1alpha1.OriginPoolTLS) json.RawMessage {
	wire := xcOriginPoolTLS{
		SNI:                    tls.SNI,
		VolterraTrustedCA:      tls.VolterraTrustedCA,
		TrustedCAURL:           tls.TrustedCAURL,
		DisableSNI:             tls.DisableSNI,
		UseServerVerification:  tls.UseServerVerification,
		SkipServerVerification: tls.SkipServerVerification,
		NoMTLS:                 tls.NoMTLS,
	}
	wire.TLSConfig = &xcTLSConfig{
		DefaultSecurity: tls.DefaultSecurity,
		LowSecurity:     tls.LowSecurity,
		MediumSecurity:  tls.MediumSecurity,
		CustomSecurity:  mapXCCustomTLSSecurity(tls.CustomSecurity),
	}
	return marshalJSON(wire)
}

func mapNetworkChoice(inside, outside *v1alpha1.EmptyObject) (json.RawMessage, json.RawMessage) {
	if inside != nil {
		return emptyObjectJSON, nil
	}
	if outside != nil {
		return nil, emptyObjectJSON
	}
	return nil, nil
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
