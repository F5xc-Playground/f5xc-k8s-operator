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

	if spec.AllowAllRequests != nil {
		out.AllowAllRequests = emptyObjectJSON
	}
	if spec.AllowList != nil {
		out.AllowList = mapPolicyAllowDenyList(spec.AllowList)
	}
	if spec.DenyAllRequests != nil {
		out.DenyAllRequests = emptyObjectJSON
	}
	if spec.DenyList != nil {
		out.DenyList = mapPolicyAllowDenyList(spec.DenyList)
	}
	if spec.RuleList != nil {
		out.RuleList = mapPolicyRuleList(spec.RuleList)
	}
	if spec.AnyServer != nil {
		out.AnyServer = emptyObjectJSON
	}
	out.ServerName = spec.ServerName
	if spec.ServerNameMatcher != nil {
		out.ServerNameMatcher = marshalJSON(xcServerNameMatcher{
			ExactValues: spec.ServerNameMatcher.ExactValues,
			RegexValues: spec.ServerNameMatcher.RegexValues,
		})
	}
	if spec.ServerSelector != nil {
		out.ServerSelector = marshalJSON(xcServerSelector{
			Expressions: spec.ServerSelector.Expressions,
		})
	}

	return out
}

func mapPolicyAllowDenyList(list *v1alpha1.PolicyAllowDenyList) json.RawMessage {
	wire := xcPolicyAllowDenyList{
		Prefixes:                list.Prefixes,
		IPPrefixSet:             mapXCObjectRefs(list.IPPrefixSet),
		ASNSet:                  mapXCObjectRefs(list.ASNSet),
		CountryList:             list.CountryList,
		DefaultActionNextPolicy: list.DefaultActionNextPolicy,
		DefaultActionDeny:       list.DefaultActionDeny,
		DefaultActionAllow:      list.DefaultActionAllow,
	}
	if list.ASNList != nil {
		wire.ASNList = &xcASNList{ASNumbers: list.ASNList.ASNumbers}
	}
	return marshalJSON(wire)
}

func mapPolicyRuleList(rl *v1alpha1.PolicyRuleList) json.RawMessage {
	var rules []xcPolicyRule
	for _, r := range rl.Rules {
		xr := xcPolicyRule{Metadata: r.Metadata}
		if r.Spec != nil {
			xrs := &xcPolicyRuleSpec{
				Action:     r.Spec.Action,
				AnyClient:  r.Spec.AnyClient,
				ClientName: r.Spec.ClientName,
				Headers:    mapXCHeaderMatchers(r.Spec.Headers),
			}
			if r.Spec.ClientSelector != nil {
				xrs.ClientSelector = &xcLabelSelector{Expressions: r.Spec.ClientSelector.Expressions}
			}
			if r.Spec.IPMatcher != nil {
				xrs.IPMatcher = &xcIPMatcher{Prefixes: r.Spec.IPMatcher.Prefixes, InvertMatch: r.Spec.IPMatcher.InvertMatch}
			}
			if r.Spec.ASNMatcher != nil {
				xrs.ASNMatcher = &xcASNMatcher{ASNumbers: r.Spec.ASNMatcher.ASNumbers}
			}
			if r.Spec.Path != nil {
				pm := mapXCPathMatcher(*r.Spec.Path)
				xrs.Path = &pm
			}
			if r.Spec.HTTPMethod != nil {
				xrs.HTTPMethod = &xcHTTPMethodMatcher{Methods: r.Spec.HTTPMethod.Methods, InvertMatcher: r.Spec.HTTPMethod.InvertMatcher}
			}
			xr.Spec = xrs
		}
		rules = append(rules, xr)
	}
	return marshalJSON(xcPolicyRuleList{Rules: rules})
}

// ---------------------------------------------------------------------------
// Wire types for ServicePolicy (snake_case JSON tags)
// ---------------------------------------------------------------------------

type xcPolicyAllowDenyList struct {
	Prefixes                []string              `json:"prefixes,omitempty"`
	IPPrefixSet             []xcObjectRef         `json:"ip_prefix_set,omitempty"`
	ASNList                 *xcASNList            `json:"asn_list,omitempty"`
	ASNSet                  []xcObjectRef         `json:"asn_set,omitempty"`
	CountryList             []string              `json:"country_list,omitempty"`
	DefaultActionNextPolicy *v1alpha1.EmptyObject `json:"default_action_next_policy,omitempty"`
	DefaultActionDeny       *v1alpha1.EmptyObject `json:"default_action_deny,omitempty"`
	DefaultActionAllow      *v1alpha1.EmptyObject `json:"default_action_allow,omitempty"`
}

type xcASNList struct {
	ASNumbers []uint32 `json:"as_numbers"`
}

type xcPolicyRuleList struct {
	Rules []xcPolicyRule `json:"rules"`
}

type xcPolicyRule struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Spec     *xcPolicyRuleSpec `json:"spec,omitempty"`
}

type xcPolicyRuleSpec struct {
	Action         string                `json:"action,omitempty"`
	AnyClient      *v1alpha1.EmptyObject `json:"any_client,omitempty"`
	ClientName     string                `json:"client_name,omitempty"`
	ClientSelector *xcLabelSelector      `json:"client_selector,omitempty"`
	IPMatcher      *xcIPMatcher          `json:"ip_matcher,omitempty"`
	ASNMatcher     *xcASNMatcher         `json:"asn_matcher,omitempty"`
	Path           *xcPathMatcher        `json:"path,omitempty"`
	Headers        []xcHeaderMatcher     `json:"headers,omitempty"`
	HTTPMethod     *xcHTTPMethodMatcher  `json:"http_method,omitempty"`
}

type xcServerNameMatcher struct {
	ExactValues []string `json:"exact_values,omitempty"`
	RegexValues []string `json:"regex_values,omitempty"`
}

type xcServerSelector struct {
	Expressions []string `json:"expressions,omitempty"`
}
