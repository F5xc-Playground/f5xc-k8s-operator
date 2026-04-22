package controller

import (
	"encoding/json"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

type xcAPIOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func buildAPIDefinitionCreate(cr *v1alpha1.APIDefinition, xcNamespace string) *xcclient.APIDefinitionCreate {
	return &xcclient.APIDefinitionCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapAPIDefinitionSpec(&cr.Spec),
	}
}

func buildAPIDefinitionReplace(cr *v1alpha1.APIDefinition, xcNamespace, resourceVersion string) *xcclient.APIDefinitionReplace {
	return &xcclient.APIDefinitionReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapAPIDefinitionSpec(&cr.Spec),
	}
}

func buildAPIDefinitionDesiredSpecJSON(cr *v1alpha1.APIDefinition, xcNamespace string) (json.RawMessage, error) {
	create := buildAPIDefinitionCreate(cr, xcNamespace)
	return json.Marshal(create.Spec)
}

func mapAPIDefinitionSpec(spec *v1alpha1.APIDefinitionSpec) xcclient.APIDefinitionSpec {
	var out xcclient.APIDefinitionSpec

	if len(spec.SwaggerSpecs) > 0 {
		out.SwaggerSpecs = marshalJSON(spec.SwaggerSpecs)
	}

	if len(spec.APIInventoryInclusionList) > 0 {
		out.APIInventoryInclusionList = marshalJSON(mapXCAPIOperations(spec.APIInventoryInclusionList))
	}
	if len(spec.APIInventoryExclusionList) > 0 {
		out.APIInventoryExclusionList = marshalJSON(mapXCAPIOperations(spec.APIInventoryExclusionList))
	}
	if len(spec.NonAPIEndpoints) > 0 {
		out.NonAPIEndpoints = marshalJSON(mapXCAPIOperations(spec.NonAPIEndpoints))
	}

	if spec.MixedSchemaOrigin != nil {
		out.MixedSchemaOrigin = emptyObjectJSON
	}
	if spec.StrictSchemaOrigin != nil {
		out.StrictSchemaOrigin = emptyObjectJSON
	}

	return out
}

func mapXCAPIOperations(ops []v1alpha1.APIOperation) []xcAPIOperation {
	out := make([]xcAPIOperation, len(ops))
	for i, op := range ops {
		out[i] = xcAPIOperation{Method: op.Method, Path: op.Path}
	}
	return out
}
