package controller

import (
	"context"
	"path/filepath"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
)

func boolPtr(b bool) *bool   { return &b }
func uint32Ptr(v uint32) *uint32 { return &v }

var (
	testEnv    *envtest.Environment
	testClient client.Client
	testCfg    *rest.Config
	testCtx    context.Context
	testCancel context.CancelFunc
)

func setupSuite(t *testing.T) {
	t.Helper()
	testCtx, testCancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	var err error
	testCfg, err = testEnv.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("adding scheme: %v", err)
	}

	testClient, err = client.New(testCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	t.Cleanup(func() {
		testCancel()
		testEnv.Stop()
	})
}

func startManagerFor(t *testing.T, setup func(mgr ctrl.Manager) error) {
	t.Helper()
	mgr, err := ctrl.NewManager(testCfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Controller: config.Controller{
			SkipNameValidation: boolPtr(true),
		},
	})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}
	if err := setup(mgr); err != nil {
		t.Fatalf("setting up controller: %v", err)
	}
	go func() {
		if err := mgr.Start(testCtx); err != nil {
			t.Errorf("manager exited with error: %v", err)
		}
	}()
}

func startManager(t *testing.T, reconciler *OriginPoolReconciler) {
	t.Helper()
	startManagerFor(t, func(mgr ctrl.Manager) error {
		reconciler.Client = mgr.GetClient()
		return reconciler.SetupWithManager(mgr)
	})
}
