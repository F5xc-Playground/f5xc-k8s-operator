package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

func buildAppFirewallCreate(cr *v1alpha1.AppFirewall, xcNamespace string) *xcclient.AppFirewallCreate {
	return &xcclient.AppFirewallCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapAppFirewallSpec(&cr.Spec),
	}
}

func buildAppFirewallReplace(cr *v1alpha1.AppFirewall, xcNamespace, resourceVersion string) *xcclient.AppFirewallReplace {
	return &xcclient.AppFirewallReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapAppFirewallSpec(&cr.Spec),
	}
}

func buildAppFirewallDesiredSpecJSON(cr *v1alpha1.AppFirewall, xcNamespace string) (json.RawMessage, error) {
	create := buildAppFirewallCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapAppFirewallSpec(spec *v1alpha1.AppFirewallSpec) xcclient.AppFirewallSpec {
	out := xcclient.AppFirewallSpec{}

	if spec.DefaultDetectionSettings != nil {
		out.DefaultDetectionSettings = json.RawMessage(spec.DefaultDetectionSettings.Raw)
	}
	if spec.DetectionSettings != nil {
		out.DetectionSettings = json.RawMessage(spec.DetectionSettings.Raw)
	}
	if spec.Monitoring != nil {
		out.Monitoring = json.RawMessage(spec.Monitoring.Raw)
	}
	if spec.Blocking != nil {
		out.Blocking = json.RawMessage(spec.Blocking.Raw)
	}
	if spec.UseDefaultBlockingPage != nil {
		out.UseDefaultBlockingPage = json.RawMessage(spec.UseDefaultBlockingPage.Raw)
	}
	if spec.BlockingPage != nil {
		out.BlockingPage = json.RawMessage(spec.BlockingPage.Raw)
	}
	if spec.AllowAllResponseCodes != nil {
		out.AllowAllResponseCodes = json.RawMessage(spec.AllowAllResponseCodes.Raw)
	}
	if spec.AllowedResponseCodes != nil {
		out.AllowedResponseCodes = json.RawMessage(spec.AllowedResponseCodes.Raw)
	}
	if spec.DefaultBotSetting != nil {
		out.DefaultBotSetting = json.RawMessage(spec.DefaultBotSetting.Raw)
	}
	if spec.BotProtectionSetting != nil {
		out.BotProtectionSetting = json.RawMessage(spec.BotProtectionSetting.Raw)
	}
	if spec.DefaultAnonymization != nil {
		out.DefaultAnonymization = json.RawMessage(spec.DefaultAnonymization.Raw)
	}
	if spec.CustomAnonymization != nil {
		out.CustomAnonymization = json.RawMessage(spec.CustomAnonymization.Raw)
	}
	if spec.UseLoadbalancerSetting != nil {
		out.UseLoadbalancerSetting = json.RawMessage(spec.UseLoadbalancerSetting.Raw)
	}

	return out
}
