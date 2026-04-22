package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateOriginPoolXCNamespace_SameNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	pool := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-1", Namespace: "default"},
		Spec:       v1alpha1.OriginPoolSpec{XCNamespace: "shared-ns", Port: 443},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pool).Build()

	err := validateOriginPoolXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "shared-ns", "default", "pool-1")
	if err != nil {
		t.Errorf("expected no error for same xcNamespace, got: %v", err)
	}
}

func TestValidateOriginPoolXCNamespace_DifferentNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	pool := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-1", Namespace: "default"},
		Spec:       v1alpha1.OriginPoolSpec{XCNamespace: "other-ns", Port: 443},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pool).Build()

	err := validateOriginPoolXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "shared-ns", "default", "pool-1")
	if err == nil {
		t.Fatal("expected error for different xcNamespace, got nil")
	}

	var nsErr *xcNamespaceError
	if !errors.As(err, &nsErr) {
		t.Fatalf("expected *xcNamespaceError, got %T", err)
	}
	if nsErr.ParentKind != "HTTPLoadBalancer" {
		t.Errorf("expected ParentKind=HTTPLoadBalancer, got %s", nsErr.ParentKind)
	}
	if nsErr.RefXCNS != "other-ns" {
		t.Errorf("expected RefXCNS=other-ns, got %s", nsErr.RefXCNS)
	}
}

func TestValidateOriginPoolXCNamespace_SharedNamespaceNotAllowed(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	pool := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "pool-1", Namespace: "default"},
		Spec:       v1alpha1.OriginPoolSpec{XCNamespace: "shared", Port: 443},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pool).Build()

	err := validateOriginPoolXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "app-ns", "default", "pool-1")
	if err == nil {
		t.Fatal("expected error when OriginPool is in 'shared' xcNamespace, got nil")
	}

	var nsErr *xcNamespaceError
	if !errors.As(err, &nsErr) {
		t.Fatalf("expected *xcNamespaceError, got %T", err)
	}
	if nsErr.RefXCNS != "shared" {
		t.Errorf("expected RefXCNS=shared, got %s", nsErr.RefXCNS)
	}
}

func TestValidateOriginPoolXCNamespace_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := validateOriginPoolXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "shared-ns", "default", "nonexistent")
	if err != nil {
		t.Errorf("expected no error when pool doesn't exist, got: %v", err)
	}
}

func TestValidateAppFirewallXCNamespace_SameNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	fw := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "fw-1", Namespace: "default"},
		Spec:       v1alpha1.AppFirewallSpec{XCNamespace: "shared-ns"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fw).Build()

	err := validateAppFirewallXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "shared-ns", "default", "fw-1")
	if err != nil {
		t.Errorf("expected no error for same xcNamespace, got: %v", err)
	}
}

func TestValidateAppFirewallXCNamespace_DifferentNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	fw := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "fw-1", Namespace: "default"},
		Spec:       v1alpha1.AppFirewallSpec{XCNamespace: "other-ns"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fw).Build()

	err := validateAppFirewallXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "shared-ns", "default", "fw-1")
	if err == nil {
		t.Fatal("expected error for different xcNamespace, got nil")
	}

	var nsErr *xcNamespaceError
	if !errors.As(err, &nsErr) {
		t.Fatalf("expected *xcNamespaceError, got %T", err)
	}
	if nsErr.RefKind != "AppFirewall" {
		t.Errorf("expected RefKind=AppFirewall, got %s", nsErr.RefKind)
	}
}

func TestValidateAppFirewallXCNamespace_SharedNamespaceAllowed(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	fw := &v1alpha1.AppFirewall{
		ObjectMeta: metav1.ObjectMeta{Name: "fw-1", Namespace: "default"},
		Spec:       v1alpha1.AppFirewallSpec{XCNamespace: "shared"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fw).Build()

	err := validateAppFirewallXCNamespace(context.Background(), c, "HTTPLoadBalancer", "my-hlb", "app-ns", "default", "fw-1")
	if err != nil {
		t.Errorf("expected no error when ref is in 'shared' xcNamespace, got: %v", err)
	}
}

func TestValidateHealthCheckXCNamespace_SameNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	hc := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "hc-1", Namespace: "default"},
		Spec:       v1alpha1.HealthCheckSpec{XCNamespace: "shared-ns"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc).Build()

	err := validateHealthCheckXCNamespace(context.Background(), c, "OriginPool", "my-pool", "shared-ns", "default", "hc-1")
	if err != nil {
		t.Errorf("expected no error for same xcNamespace, got: %v", err)
	}
}

func TestValidateHealthCheckXCNamespace_DifferentNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	hc := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "hc-1", Namespace: "default"},
		Spec:       v1alpha1.HealthCheckSpec{XCNamespace: "other-ns"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc).Build()

	err := validateHealthCheckXCNamespace(context.Background(), c, "OriginPool", "my-pool", "shared-ns", "default", "hc-1")
	if err == nil {
		t.Fatal("expected error for different xcNamespace, got nil")
	}

	var nsErr *xcNamespaceError
	if !errors.As(err, &nsErr) {
		t.Fatalf("expected *xcNamespaceError, got %T", err)
	}
	if nsErr.RefKind != "HealthCheck" {
		t.Errorf("expected RefKind=HealthCheck, got %s", nsErr.RefKind)
	}
}

func TestValidateHealthCheckXCNamespace_SharedNamespaceAllowed(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	hc := &v1alpha1.HealthCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "hc-1", Namespace: "default"},
		Spec:       v1alpha1.HealthCheckSpec{XCNamespace: "shared"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hc).Build()

	err := validateHealthCheckXCNamespace(context.Background(), c, "OriginPool", "my-pool", "app-ns", "default", "hc-1")
	if err != nil {
		t.Errorf("expected no error when ref is in 'shared' xcNamespace, got: %v", err)
	}
}

func TestValidateHealthCheckXCNamespace_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := validateHealthCheckXCNamespace(context.Background(), c, "OriginPool", "my-pool", "shared-ns", "default", "nonexistent")
	if err != nil {
		t.Errorf("expected no error when healthcheck doesn't exist, got: %v", err)
	}
}
