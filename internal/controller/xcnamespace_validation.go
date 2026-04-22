package controller

import (
	"context"
	"fmt"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type xcNamespaceError struct {
	ParentKind string
	ParentName string
	ParentXCNS string
	RefKind    string
	RefName    string
	RefXCNS    string
}

func (e *xcNamespaceError) Error() string {
	return fmt.Sprintf(
		"%s %q (xcNamespace %q) references %s %q which is in xcNamespace %q; cross-namespace references are not allowed",
		e.ParentKind, e.ParentName, e.ParentXCNS, e.RefKind, e.RefName, e.RefXCNS,
	)
}

// validateOriginPoolXCNamespace checks that a referenced OriginPool has the same xcNamespace.
// It looks up the OriginPool CR in the same K8s namespace. If the pool CR doesn't exist
// or has no xcNamespace set yet, it returns nil (validation passes — the pool's own
// controller will enforce its xcNamespace).
func validateOriginPoolXCNamespace(ctx context.Context, c client.Client, parentKind, parentName, parentXCNS, k8sNamespace, poolName string) error {
	var pool v1alpha1.OriginPool
	key := client.ObjectKey{Namespace: k8sNamespace, Name: poolName}
	if err := c.Get(ctx, key, &pool); err != nil {
		return nil // pool doesn't exist yet or isn't accessible — skip validation
	}
	if pool.Spec.XCNamespace != "" && pool.Spec.XCNamespace != parentXCNS {
		return &xcNamespaceError{
			ParentKind: parentKind, ParentName: parentName, ParentXCNS: parentXCNS,
			RefKind: "OriginPool", RefName: poolName, RefXCNS: pool.Spec.XCNamespace,
		}
	}
	return nil
}

// validateAppFirewallXCNamespace checks that a referenced AppFirewall has the same xcNamespace.
func validateAppFirewallXCNamespace(ctx context.Context, c client.Client, parentKind, parentName, parentXCNS, k8sNamespace, fwName string) error {
	var fw v1alpha1.AppFirewall
	key := client.ObjectKey{Namespace: k8sNamespace, Name: fwName}
	if err := c.Get(ctx, key, &fw); err != nil {
		return nil
	}
	if fw.Spec.XCNamespace != "" && fw.Spec.XCNamespace != parentXCNS && fw.Spec.XCNamespace != "shared" {
		return &xcNamespaceError{
			ParentKind: parentKind, ParentName: parentName, ParentXCNS: parentXCNS,
			RefKind: "AppFirewall", RefName: fwName, RefXCNS: fw.Spec.XCNamespace,
		}
	}
	return nil
}

// validateHealthCheckXCNamespace checks that a referenced HealthCheck has the same xcNamespace.
func validateHealthCheckXCNamespace(ctx context.Context, c client.Client, parentKind, parentName, parentXCNS, k8sNamespace, hcName string) error {
	var hc v1alpha1.HealthCheck
	key := client.ObjectKey{Namespace: k8sNamespace, Name: hcName}
	if err := c.Get(ctx, key, &hc); err != nil {
		return nil
	}
	if hc.Spec.XCNamespace != "" && hc.Spec.XCNamespace != parentXCNS && hc.Spec.XCNamespace != "shared" {
		return &xcNamespaceError{
			ParentKind: parentKind, ParentName: parentName, ParentXCNS: parentXCNS,
			RefKind: "HealthCheck", RefName: hcName, RefXCNS: hc.Spec.XCNamespace,
		}
	}
	return nil
}
