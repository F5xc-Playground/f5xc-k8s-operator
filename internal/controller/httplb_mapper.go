package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildHTTPLoadBalancerCreate(cr *v1alpha1.HTTPLoadBalancer, xcNamespace string) *xcclient.HTTPLoadBalancerCreate {
	return &xcclient.HTTPLoadBalancerCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapHTTPLoadBalancerSpec(&cr.Spec),
	}
}

func buildHTTPLoadBalancerReplace(cr *v1alpha1.HTTPLoadBalancer, xcNamespace, resourceVersion string) *xcclient.HTTPLoadBalancerReplace {
	return &xcclient.HTTPLoadBalancerReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapHTTPLoadBalancerSpec(&cr.Spec),
	}
}

func buildHTTPLoadBalancerDesiredSpecJSON(cr *v1alpha1.HTTPLoadBalancer, xcNamespace string) (json.RawMessage, error) {
	create := buildHTTPLoadBalancerCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapHTTPLoadBalancerSpec(spec *v1alpha1.HTTPLoadBalancerSpec) xcclient.HTTPLoadBalancerSpec {
	var out xcclient.HTTPLoadBalancerSpec
	out.Domains = spec.Domains

	for i := range spec.DefaultRoutePools {
		out.DefaultRoutePools = append(out.DefaultRoutePools, mapRoutePool(&spec.DefaultRoutePools[i]))
	}

	if len(spec.Routes) > 0 {
		routesJSON, _ := json.Marshal(spec.Routes)
		out.Routes = routesJSON
	}

	// TLS OneOf
	if spec.HTTP != nil {
		out.HTTP = json.RawMessage(spec.HTTP.Raw)
	}
	if spec.HTTPS != nil {
		out.HTTPS = json.RawMessage(spec.HTTPS.Raw)
	}
	if spec.HTTPSAutoCert != nil {
		out.HTTPSAutoCert = json.RawMessage(spec.HTTPSAutoCert.Raw)
	}

	// WAF OneOf
	if spec.DisableWAF != nil {
		out.DisableWAF = json.RawMessage(spec.DisableWAF.Raw)
	}
	if spec.AppFirewall != nil {
		out.AppFirewall = mapObjectRefPtr(spec.AppFirewall)
	}

	// Bot defense OneOf
	if spec.DisableBotDefense != nil {
		out.DisableBotDefense = json.RawMessage(spec.DisableBotDefense.Raw)
	}
	if spec.BotDefense != nil {
		out.BotDefense = json.RawMessage(spec.BotDefense.Raw)
	}

	// API discovery OneOf
	if spec.DisableAPIDiscovery != nil {
		out.DisableAPIDiscovery = json.RawMessage(spec.DisableAPIDiscovery.Raw)
	}
	if spec.EnableAPIDiscovery != nil {
		out.EnableAPIDiscovery = json.RawMessage(spec.EnableAPIDiscovery.Raw)
	}

	// IP reputation OneOf
	if spec.DisableIPReputation != nil {
		out.DisableIPReputation = json.RawMessage(spec.DisableIPReputation.Raw)
	}
	if spec.EnableIPReputation != nil {
		out.EnableIPReputation = json.RawMessage(spec.EnableIPReputation.Raw)
	}

	// Rate limit OneOf
	if spec.DisableRateLimit != nil {
		out.DisableRateLimit = json.RawMessage(spec.DisableRateLimit.Raw)
	}
	if spec.RateLimit != nil {
		out.RateLimit = json.RawMessage(spec.RateLimit.Raw)
	}

	// Challenge OneOf
	if spec.NoChallenge != nil {
		out.NoChallenge = json.RawMessage(spec.NoChallenge.Raw)
	}
	if spec.JSChallenge != nil {
		out.JSChallenge = json.RawMessage(spec.JSChallenge.Raw)
	}
	if spec.CaptchaChallenge != nil {
		out.CaptchaChallenge = json.RawMessage(spec.CaptchaChallenge.Raw)
	}
	if spec.PolicyBasedChallenge != nil {
		out.PolicyBasedChallenge = json.RawMessage(spec.PolicyBasedChallenge.Raw)
	}

	// LB algorithm OneOf
	if spec.RoundRobin != nil {
		out.RoundRobin = json.RawMessage(spec.RoundRobin.Raw)
	}
	if spec.LeastActive != nil {
		out.LeastActive = json.RawMessage(spec.LeastActive.Raw)
	}
	if spec.Random != nil {
		out.Random = json.RawMessage(spec.Random.Raw)
	}
	if spec.SourceIPStickiness != nil {
		out.SourceIPStickiness = json.RawMessage(spec.SourceIPStickiness.Raw)
	}
	if spec.CookieStickiness != nil {
		out.CookieStickiness = json.RawMessage(spec.CookieStickiness.Raw)
	}
	if spec.RingHash != nil {
		out.RingHash = json.RawMessage(spec.RingHash.Raw)
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

	// Service policies OneOf
	if spec.ServicePoliciesFromNamespace != nil {
		out.ServicePoliciesFromNamespace = json.RawMessage(spec.ServicePoliciesFromNamespace.Raw)
	}
	if spec.ActiveServicePolicies != nil {
		out.ActiveServicePolicies = json.RawMessage(spec.ActiveServicePolicies.Raw)
	}
	if spec.NoServicePolicies != nil {
		out.NoServicePolicies = json.RawMessage(spec.NoServicePolicies.Raw)
	}

	// User ID OneOf
	if spec.UserIDClientIP != nil {
		out.UserIDClientIP = json.RawMessage(spec.UserIDClientIP.Raw)
	}

	return out
}
