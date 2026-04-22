package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

// ---------------------------------------------------------------------------
// Wire-format types (snake_case JSON tags — mirror F5XC REST API)
// ---------------------------------------------------------------------------

type xcDetectionSettings struct {
	SignatureSelectionSetting *xcSignatureSelectionSetting `json:"signature_selection_setting,omitempty"`
	EnableSuppression        *v1alpha1.EmptyObject        `json:"enable_suppression,omitempty"`
	DisableSuppression       *v1alpha1.EmptyObject        `json:"disable_suppression,omitempty"`
	EnableThreatCampaigns    *v1alpha1.EmptyObject        `json:"enable_threat_campaigns,omitempty"`
	DisableThreatCampaigns   *v1alpha1.EmptyObject        `json:"disable_threat_campaigns,omitempty"`
}

type xcSignatureSelectionSetting struct {
	DefaultAttackTypeSettings       *v1alpha1.EmptyObject `json:"default_attack_type_settings,omitempty"`
	AttackTypeSettings              *xcAttackTypeSettings `json:"attack_type_settings,omitempty"`
	HighMediumLowAccuracySignatures *v1alpha1.EmptyObject `json:"high_medium_low_accuracy_signatures,omitempty"`
	HighMediumAccuracySignatures    *v1alpha1.EmptyObject `json:"high_medium_accuracy_signatures,omitempty"`
	OnlyHighAccuracySignatures      *v1alpha1.EmptyObject `json:"only_high_accuracy_signatures,omitempty"`
}

type xcAttackTypeSettings struct {
	DisabledAttackTypes []xcAttackType `json:"disabled_attack_types,omitempty"`
}

type xcAttackType struct {
	Name string `json:"name"`
}

type xcBlockingPage struct {
	BlockingPage string `json:"blocking_page,omitempty"`
	ResponseCode string `json:"response_code,omitempty"`
}

type xcAllowedResponseCodes struct {
	ResponseCode []int `json:"response_code"`
}

type xcBotProtectionSetting struct {
	MaliciousBotAction  string `json:"malicious_bot_action,omitempty"`
	SuspiciousBotAction string `json:"suspicious_bot_action,omitempty"`
	GoodBotAction       string `json:"good_bot_action,omitempty"`
}

type xcCustomAnonymization struct {
	AnonymizationConfig []xcAnonymizationEntry `json:"anonymization_config,omitempty"`
	SpecificDomains     []string               `json:"specific_domains,omitempty"`
}

type xcAnonymizationEntry struct {
	HeaderName     string `json:"header_name,omitempty"`
	QueryParameter string `json:"query_parameter,omitempty"`
	CookieName     string `json:"cookie_name,omitempty"`
}

// ---------------------------------------------------------------------------
// Builder functions
// ---------------------------------------------------------------------------

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
		out.DefaultDetectionSettings = emptyObjectJSON
	}
	if spec.DetectionSettings != nil {
		out.DetectionSettings = mapDetectionSettings(spec.DetectionSettings)
	}
	if spec.Monitoring != nil {
		out.Monitoring = emptyObjectJSON
	}
	if spec.Blocking != nil {
		out.Blocking = emptyObjectJSON
	}
	if spec.UseDefaultBlockingPage != nil {
		out.UseDefaultBlockingPage = emptyObjectJSON
	}
	if spec.BlockingPage != nil {
		out.BlockingPage = marshalJSON(xcBlockingPage{
			BlockingPage: spec.BlockingPage.BlockingPage,
			ResponseCode: spec.BlockingPage.ResponseCode,
		})
	}
	if spec.AllowAllResponseCodes != nil {
		out.AllowAllResponseCodes = emptyObjectJSON
	}
	if spec.AllowedResponseCodes != nil {
		out.AllowedResponseCodes = marshalJSON(xcAllowedResponseCodes{
			ResponseCode: spec.AllowedResponseCodes.ResponseCode,
		})
	}
	if spec.DefaultBotSetting != nil {
		out.DefaultBotSetting = emptyObjectJSON
	}
	if spec.BotProtectionSetting != nil {
		out.BotProtectionSetting = marshalJSON(xcBotProtectionSetting{
			MaliciousBotAction:  spec.BotProtectionSetting.MaliciousBotAction,
			SuspiciousBotAction: spec.BotProtectionSetting.SuspiciousBotAction,
			GoodBotAction:       spec.BotProtectionSetting.GoodBotAction,
		})
	}
	if spec.DefaultAnonymization != nil {
		out.DefaultAnonymization = emptyObjectJSON
	}
	if spec.DisableAnonymization != nil {
		out.DisableAnonymization = emptyObjectJSON
	}
	if spec.CustomAnonymization != nil {
		out.CustomAnonymization = mapCustomAnonymization(spec.CustomAnonymization)
	}

	return out
}

func mapDetectionSettings(ds *v1alpha1.DetectionSettings) json.RawMessage {
	wire := xcDetectionSettings{
		EnableSuppression:      ds.EnableSuppression,
		DisableSuppression:     ds.DisableSuppression,
		EnableThreatCampaigns:  ds.EnableThreatCampaigns,
		DisableThreatCampaigns: ds.DisableThreatCampaigns,
	}
	if ds.SignatureSelectionSetting != nil {
		wire.SignatureSelectionSetting = &xcSignatureSelectionSetting{
			DefaultAttackTypeSettings:       ds.SignatureSelectionSetting.DefaultAttackTypeSettings,
			HighMediumLowAccuracySignatures: ds.SignatureSelectionSetting.HighMediumLowAccuracySignatures,
			HighMediumAccuracySignatures:    ds.SignatureSelectionSetting.HighMediumAccuracySignatures,
			OnlyHighAccuracySignatures:      ds.SignatureSelectionSetting.OnlyHighAccuracySignatures,
		}
		if ds.SignatureSelectionSetting.AttackTypeSettings != nil {
			var disabled []xcAttackType
			for _, at := range ds.SignatureSelectionSetting.AttackTypeSettings.DisabledAttackTypes {
				disabled = append(disabled, xcAttackType{Name: at.Name})
			}
			wire.SignatureSelectionSetting.AttackTypeSettings = &xcAttackTypeSettings{
				DisabledAttackTypes: disabled,
			}
		}
	}
	return marshalJSON(wire)
}

func mapCustomAnonymization(ca *v1alpha1.CustomAnonymization) json.RawMessage {
	wire := xcCustomAnonymization{SpecificDomains: ca.SpecificDomains}
	for _, e := range ca.AnonymizationConfig {
		wire.AnonymizationConfig = append(wire.AnonymizationConfig, xcAnonymizationEntry{
			HeaderName:     e.HeaderName,
			QueryParameter: e.QueryParameter,
			CookieName:     e.CookieName,
		})
	}
	return marshalJSON(wire)
}
