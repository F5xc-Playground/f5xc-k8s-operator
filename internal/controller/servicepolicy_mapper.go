package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildServicePolicyCreate(cr *v1alpha1.ServicePolicy, xcNamespace string) *xcclient.ServicePolicyCreate {
	return &xcclient.ServicePolicyCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapServicePolicySpec(&cr.Spec),
	}
}

func buildServicePolicyReplace(cr *v1alpha1.ServicePolicy, xcNamespace, resourceVersion string) *xcclient.ServicePolicyReplace {
	return &xcclient.ServicePolicyReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapServicePolicySpec(&cr.Spec),
	}
}

func buildServicePolicyDesiredSpecJSON(cr *v1alpha1.ServicePolicy, xcNamespace string) (json.RawMessage, error) {
	create := buildServicePolicyCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapServicePolicySpec(spec *v1alpha1.ServicePolicySpec) xcclient.ServicePolicySpec {
	var out xcclient.ServicePolicySpec

	// Rule choice OneOf
	if spec.AllowAllRequests != nil {
		out.AllowAllRequests = json.RawMessage(spec.AllowAllRequests.Raw)
	}
	if spec.AllowList != nil {
		out.AllowList = json.RawMessage(spec.AllowList.Raw)
	}
	if spec.DenyAllRequests != nil {
		out.DenyAllRequests = json.RawMessage(spec.DenyAllRequests.Raw)
	}
	if spec.DenyList != nil {
		out.DenyList = json.RawMessage(spec.DenyList.Raw)
	}
	if spec.RuleList != nil {
		out.RuleList = json.RawMessage(spec.RuleList.Raw)
	}

	// Server choice OneOf
	if spec.AnyServer != nil {
		out.AnyServer = json.RawMessage(spec.AnyServer.Raw)
	}
	out.ServerName = spec.ServerName
	if spec.ServerNameMatcher != nil {
		out.ServerNameMatcher = json.RawMessage(spec.ServerNameMatcher.Raw)
	}
	if spec.ServerSelector != nil {
		out.ServerSelector = json.RawMessage(spec.ServerSelector.Raw)
	}

	return out
}
