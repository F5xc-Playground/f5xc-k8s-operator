package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildHealthCheckCreate(cr *v1alpha1.HealthCheck, xcNamespace string) xcclient.CreateHealthCheck {
	return xcclient.CreateHealthCheck{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapHealthCheckSpec(&cr.Spec),
	}
}

func buildHealthCheckReplace(cr *v1alpha1.HealthCheck, xcNamespace, resourceVersion string) xcclient.ReplaceHealthCheck {
	return xcclient.ReplaceHealthCheck{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapHealthCheckSpec(&cr.Spec),
	}
}

func buildHealthCheckDesiredSpecJSON(cr *v1alpha1.HealthCheck, xcNamespace string) (json.RawMessage, error) {
	create := buildHealthCheckCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapHealthCheckSpec(spec *v1alpha1.HealthCheckSpec) xcclient.HealthCheckSpec {
	var out xcclient.HealthCheckSpec

	if spec.HTTPHealthCheck != nil {
		out.HTTPHealthCheck = &xcclient.HTTPHealthCheck{
			Path:                spec.HTTPHealthCheck.Path,
			UseHTTP2:            spec.HTTPHealthCheck.UseHTTP2,
			ExpectedStatusCodes: spec.HTTPHealthCheck.ExpectedStatusCodes,
		}
	}

	if spec.TCPHealthCheck != nil {
		out.TCPHealthCheck = &xcclient.TCPHealthCheck{
			Send:    spec.TCPHealthCheck.Send,
			Receive: spec.TCPHealthCheck.Receive,
		}
	}

	if spec.HealthyThreshold != nil {
		out.HealthyThreshold = *spec.HealthyThreshold
	}
	if spec.UnhealthyThreshold != nil {
		out.UnhealthyThreshold = *spec.UnhealthyThreshold
	}
	if spec.Interval != nil {
		out.Interval = *spec.Interval
	}
	if spec.Timeout != nil {
		out.Timeout = *spec.Timeout
	}
	if spec.JitterPercent != nil {
		out.JitterPercent = *spec.JitterPercent
	}

	return out
}
