package v1alpha1

type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Tenant    string `json:"tenant,omitempty"`
}

type RoutePool struct {
	Pool     ObjectRef `json:"pool"`
	Weight   *uint32   `json:"weight,omitempty"`
	Priority *uint32   `json:"priority,omitempty"`
}
