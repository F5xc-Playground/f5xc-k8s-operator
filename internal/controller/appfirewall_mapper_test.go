package controller

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleAppFirewall(name, namespace string) *v1alpha1.AppFirewall {
	return &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.AppFirewallSpec{
			XCNamespace: namespace,
			Blocking:    &v1alpha1.EmptyObject{},
		},
	}
}

func TestBuildAppFirewallCreate_BasicFields(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "default")

	result := buildAppFirewallCreate(cr, "default")
	assert.Equal(t, "my-afw", result.Metadata.Name)
	assert.Equal(t, "default", result.Metadata.Namespace)
	assert.JSONEq(t, "{}", string(result.Spec.Blocking))
	assert.Nil(t, result.Spec.Monitoring)
}

func TestBuildAppFirewallCreate_MultipleOneOfGroups(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-multi", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking:               &v1alpha1.EmptyObject{},
			UseDefaultBlockingPage: &v1alpha1.EmptyObject{},
			DefaultBotSetting:      &v1alpha1.EmptyObject{},
			DefaultAnonymization:   &v1alpha1.EmptyObject{},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{}`, string(result.Spec.Blocking))
	assert.JSONEq(t, `{}`, string(result.Spec.UseDefaultBlockingPage))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultBotSetting))
	assert.JSONEq(t, `{}`, string(result.Spec.DefaultAnonymization))
	assert.Nil(t, result.Spec.Monitoring)
	assert.Nil(t, result.Spec.BlockingPage)
}

func TestBuildAppFirewallReplace_IncludesResourceVersion(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "ns")

	result := buildAppFirewallReplace(cr, "ns", "rv-5")
	assert.Equal(t, "rv-5", result.Metadata.ResourceVersion)
	assert.JSONEq(t, "{}", string(result.Spec.Blocking))
}

func TestBuildAppFirewallCreate_XCNamespaceOverride(t *testing.T) {
	cr := sampleAppFirewall("my-afw", "k8s-ns")

	result := buildAppFirewallCreate(cr, "xc-override")
	assert.Equal(t, "xc-override", result.Metadata.Namespace)
}

func TestBuildAppFirewallDesiredSpecJSON(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "my-afw", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &v1alpha1.EmptyObject{},
		},
	}

	raw, err := buildAppFirewallDesiredSpecJSON(cr, "ns")
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	_, hasBlocking := spec["blocking"]
	assert.True(t, hasBlocking)
	_, hasMetadata := spec["metadata"]
	assert.False(t, hasMetadata, "spec JSON must not contain metadata")
}

func TestBuildAppFirewallCreate_DetectionSettings(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-detect", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			DetectionSettings: &v1alpha1.DetectionSettings{
				EnableSuppression:     &v1alpha1.EmptyObject{},
				EnableThreatCampaigns: &v1alpha1.EmptyObject{},
				SignatureSelectionSetting: &v1alpha1.SignatureSelectionSetting{
					DefaultAttackTypeSettings:  &v1alpha1.EmptyObject{},
					OnlyHighAccuracySignatures: &v1alpha1.EmptyObject{},
				},
			},
			Blocking: &v1alpha1.EmptyObject{},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.NotNil(t, result.Spec.DetectionSettings)
	assert.Nil(t, result.Spec.DefaultDetectionSettings)

	var ds map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Spec.DetectionSettings, &ds))
	_, hasEnable := ds["enable_suppression"]
	assert.True(t, hasEnable)
}

func TestBuildAppFirewallCreate_BotProtectionSetting(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-bot", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking: &v1alpha1.EmptyObject{},
			BotProtectionSetting: &v1alpha1.BotProtectionSetting{
				MaliciousBotAction:  "BLOCK",
				SuspiciousBotAction: "FLAG",
				GoodBotAction:       "REPORT",
			},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{"malicious_bot_action":"BLOCK","suspicious_bot_action":"FLAG","good_bot_action":"REPORT"}`, string(result.Spec.BotProtectionSetting))
}

func TestBuildAppFirewallCreate_AllowedResponseCodes(t *testing.T) {
	cr := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "afw-codes", Namespace: "ns"},
		Spec: v1alpha1.AppFirewallSpec{
			Blocking:             &v1alpha1.EmptyObject{},
			AllowedResponseCodes: &v1alpha1.AllowedResponseCodes{ResponseCode: []int{200, 201, 204}},
		},
	}

	result := buildAppFirewallCreate(cr, "ns")
	assert.JSONEq(t, `{"response_code":[200,201,204]}`, string(result.Spec.AllowedResponseCodes))
}
