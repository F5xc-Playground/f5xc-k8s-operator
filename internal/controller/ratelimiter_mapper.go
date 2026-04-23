package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildRateLimiterCreate(cr *v1alpha1.RateLimiter, xcNamespace string) xcclient.XCRateLimiterCreate {
	return xcclient.XCRateLimiterCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapRateLimiterSpec(&cr.Spec),
	}
}

func buildRateLimiterReplace(cr *v1alpha1.RateLimiter, xcNamespace, resourceVersion string) xcclient.XCRateLimiterReplace {
	return xcclient.XCRateLimiterReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapRateLimiterSpec(&cr.Spec),
	}
}

func buildRateLimiterDesiredSpecJSON(cr *v1alpha1.RateLimiter, xcNamespace string) (json.RawMessage, error) {
	create := buildRateLimiterCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapRateLimiterSpec(spec *v1alpha1.RateLimiterSpec) xcclient.XCRateLimiterSpec {
	val := xcclient.RateLimitValue{
		TotalNumber:     spec.Threshold,
		Unit:            spec.Unit,
		BurstMultiplier: 1,
	}
	if spec.BurstMultiplier != nil {
		val.BurstMultiplier = *spec.BurstMultiplier
	}
	return xcclient.XCRateLimiterSpec{
		Limits: []xcclient.RateLimitValue{val},
	}
}
