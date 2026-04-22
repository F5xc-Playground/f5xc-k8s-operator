package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcMaliciousUserMitigationType struct {
	Rules []xcMaliciousUserMitigationRule `json:"rules"`
}

type xcMaliciousUserMitigationRule struct {
	ThreatLevel      xcMaliciousUserThreatLevel      `json:"threat_level"`
	MitigationAction xcMaliciousUserMitigationAction `json:"mitigation_action"`
}

type xcMaliciousUserThreatLevel struct {
	Low    *v1alpha1.EmptyObject `json:"low,omitempty"`
	Medium *v1alpha1.EmptyObject `json:"medium,omitempty"`
	High   *v1alpha1.EmptyObject `json:"high,omitempty"`
}

type xcMaliciousUserMitigationAction struct {
	BlockTemporarily    *v1alpha1.EmptyObject `json:"block_temporarily,omitempty"`
	CaptchaChallenge    *v1alpha1.EmptyObject `json:"captcha_challenge,omitempty"`
	JavascriptChallenge *v1alpha1.EmptyObject `json:"javascript_challenge,omitempty"`
}

func buildMaliciousUserMitigationCreate(cr *v1alpha1.MaliciousUserMitigation, xcNamespace string) *xcclient.MaliciousUserMitigationCreate {
	return &xcclient.MaliciousUserMitigationCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapMaliciousUserMitigationSpec(&cr.Spec),
	}
}

func buildMaliciousUserMitigationReplace(cr *v1alpha1.MaliciousUserMitigation, xcNamespace, resourceVersion string) *xcclient.MaliciousUserMitigationReplace {
	return &xcclient.MaliciousUserMitigationReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapMaliciousUserMitigationSpec(&cr.Spec),
	}
}

func buildMaliciousUserMitigationDesiredSpecJSON(cr *v1alpha1.MaliciousUserMitigation, xcNamespace string) (json.RawMessage, error) {
	create := buildMaliciousUserMitigationCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapMaliciousUserMitigationSpec(spec *v1alpha1.MaliciousUserMitigationSpec) xcclient.MaliciousUserMitigationSpec {
	var out xcclient.MaliciousUserMitigationSpec
	if spec.MitigationType != nil {
		var rules []xcMaliciousUserMitigationRule
		for _, r := range spec.MitigationType.Rules {
			rules = append(rules, xcMaliciousUserMitigationRule{
				ThreatLevel: xcMaliciousUserThreatLevel{
					Low: r.ThreatLevel.Low, Medium: r.ThreatLevel.Medium, High: r.ThreatLevel.High,
				},
				MitigationAction: xcMaliciousUserMitigationAction{
					BlockTemporarily: r.MitigationAction.BlockTemporarily, CaptchaChallenge: r.MitigationAction.CaptchaChallenge, JavascriptChallenge: r.MitigationAction.JavascriptChallenge,
				},
			})
		}
		out.MitigationType = marshalJSON(xcMaliciousUserMitigationType{Rules: rules})
	}
	return out
}
