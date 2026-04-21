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
	out := xcclient.ServicePolicySpec{
		Algo: spec.Algo,
	}
	for _, rule := range spec.Rules {
		out.Rules = append(out.Rules, json.RawMessage(rule.Raw))
	}
	return out
}
