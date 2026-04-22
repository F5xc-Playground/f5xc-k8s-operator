// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/controller"
	"github.com/kreynolds/f5xc-k8s-operator/internal/credentials"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

func main() {
	var (
		tenantURL              string
		credentialsSecret      string
		credentialsSecretNS    string
		rateLimitRPS           float64
		rateLimitBurst         int
		metricsBindAddress     string
		healthProbeBindAddress string
		enableLeaderElection   bool
	)

	flag.StringVar(&tenantURL, "tenant-url", "", "F5 XC tenant URL (required)")
	flag.StringVar(&credentialsSecret, "credentials-secret", "xc-credentials", "Name of the Secret containing XC credentials")
	flag.StringVar(&credentialsSecretNS, "credentials-secret-namespace", "default", "Namespace of the credentials Secret")
	flag.Float64Var(&rateLimitRPS, "rate-limit-rps", 2.0, "Default XC API rate limit in requests per second")
	flag.IntVar(&rateLimitBurst, "rate-limit-burst", 5, "Default XC API rate limit burst size")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to")
	flag.StringVar(&healthProbeBindAddress, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if tenantURL == "" {
		fmt.Fprintln(os.Stderr, "error: --tenant-url is required")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("setup")

	rateLimits := xcclient.RateLimitConfig{
		DefaultRPS:   rateLimitRPS,
		DefaultBurst: rateLimitBurst,
	}

	restCfg := ctrl.GetConfigOrDie()

	// Build a direct (non-cached) client for the initial Secret read, which
	// must happen before mgr.Start() — the manager's cache-backed client is
	// not usable until the cache has synced.
	directClient, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "unable to create direct Kubernetes client")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsBindAddress,
		},
		HealthProbeBindAddress: healthProbeBindAddress,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f5xc-operator.xc.f5.com",
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	cs, initialCfg, err := buildClientSet(
		context.Background(),
		directClient,
		log,
		credentialsSecretNS,
		credentialsSecret,
		tenantURL,
		rateLimits,
	)
	if err != nil {
		log.Error(err, "unable to build XC client")
		os.Exit(1)
	}

	if err := (&controller.OriginPoolReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("OriginPool"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "OriginPool")
		os.Exit(1)
	}

	if err := (&controller.HTTPLoadBalancerReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("HTTPLoadBalancer"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "HTTPLoadBalancer")
		os.Exit(1)
	}

	if err := (&controller.TCPLoadBalancerReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("TCPLoadBalancer"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "TCPLoadBalancer")
		os.Exit(1)
	}

	if err := (&controller.AppFirewallReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("AppFirewall"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "AppFirewall")
		os.Exit(1)
	}

	if err := (&controller.HealthCheckReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("HealthCheck"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "HealthCheck")
		os.Exit(1)
	}

	if err := (&controller.ServicePolicyReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("ServicePolicy"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "ServicePolicy")
		os.Exit(1)
	}

	if err := (&controller.RateLimiterReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("RateLimiter"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "RateLimiter")
		os.Exit(1)
	}

	if err := (&controller.CertificateReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("Certificate"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "Certificate")
		os.Exit(1)
	}

	if err := (&controller.APIDefinitionReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("APIDefinition"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "APIDefinition")
		os.Exit(1)
	}

	if err := (&controller.UserIdentificationReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("UserIdentification"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "UserIdentification")
		os.Exit(1)
	}

	if err := (&controller.MaliciousUserMitigationReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("MaliciousUserMitigation"),
		ClientSet: cs,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "MaliciousUserMitigation")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up readiness check")
		os.Exit(1)
	}

	if err := setupSecretWatch(
		mgr,
		cs,
		initialCfg,
		log,
		credentialsSecretNS,
		credentialsSecret,
		tenantURL,
		rateLimits,
	); err != nil {
		log.Error(err, "unable to set up credential Secret watch")
		os.Exit(1)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// buildClientSet reads the credential Secret and constructs a ready-to-use ClientSet.
// It returns both the ClientSet and the xcclient.Config so the caller can seed
// the credential watcher's currentCfg for P12 temp-file cleanup.
func buildClientSet(
	ctx context.Context,
	k8sClient client.Client,
	log logr.Logger,
	secretNS, secretName, tenantURL string,
	rateLimits xcclient.RateLimitConfig,
) (*xcclientset.ClientSet, xcclient.Config, error) {
	var secret corev1.Secret
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: secretNS,
		Name:      secretName,
	}, &secret); err != nil {
		return nil, xcclient.Config{}, fmt.Errorf("reading credential Secret %s/%s: %w", secretNS, secretName, err)
	}

	cfg, err := credentials.ConfigFromSecret(&secret, tenantURL, rateLimits)
	if err != nil {
		return nil, xcclient.Config{}, fmt.Errorf("building XC config from Secret: %w", err)
	}

	xcClient, err := xcclient.NewClient(cfg, log.WithName("xcclient"), prometheus.DefaultRegisterer)
	if err != nil {
		return nil, xcclient.Config{}, fmt.Errorf("creating XC client: %w", err)
	}

	return xcclientset.New(xcClient), cfg, nil
}

// credentialWatcher reconciles credential Secret changes by rebuilding the XC client.
type credentialWatcher struct {
	k8sClient  client.Client
	cs         *xcclientset.ClientSet
	log        logr.Logger
	secretNS   string
	secretName string
	tenantURL  string
	rateLimits xcclient.RateLimitConfig
	currentCfg xcclient.Config // tracks the active config for P12 temp-file cleanup
}

func (w *credentialWatcher) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	var secret corev1.Secret
	if err := w.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: w.secretNS,
		Name:      w.secretName,
	}, &secret); err != nil {
		w.log.Error(err, "failed to read credential Secret; keeping existing client")
		return reconcile.Result{}, nil
	}

	cfg, err := credentials.ConfigFromSecret(&secret, w.tenantURL, w.rateLimits)
	if err != nil {
		w.log.Error(err, "failed to build XC config from updated Secret; keeping existing client")
		return reconcile.Result{}, nil
	}

	// Use a fresh isolated registry for rotated clients so that registering
	// the same metric names a second time does not panic.
	newClient, err := xcclient.NewClient(cfg, w.log.WithName("xcclient"), prometheus.NewRegistry())
	if err != nil {
		w.log.Error(err, "failed to create new XC client from updated Secret; keeping existing client")
		if cfg.CertP12Path != "" {
			os.Remove(cfg.CertP12Path)
		}
		return reconcile.Result{}, nil
	}

	// Remember the previous config so we can clean up its P12 temp file after
	// the swap succeeds.
	prevCfg := w.currentCfg
	w.cs.Swap(newClient)
	w.currentCfg = cfg
	w.log.Info("XC client rotated successfully")

	// Remove the old P12 temp file now that the new client is live.
	if prevCfg.CertP12Path != "" {
		if err := os.Remove(prevCfg.CertP12Path); err != nil && !os.IsNotExist(err) {
			w.log.Error(err, "failed to remove old P12 temp file", "path", prevCfg.CertP12Path)
		}
	}

	return reconcile.Result{}, nil
}

// setupSecretWatch registers a secondary controller that watches the credential
// Secret and calls cs.Swap whenever it changes.
func setupSecretWatch(
	mgr ctrl.Manager,
	cs *xcclientset.ClientSet,
	initialCfg xcclient.Config,
	log logr.Logger,
	secretNS, secretName, tenantURL string,
	rateLimits xcclient.RateLimitConfig,
) error {
	watcher := &credentialWatcher{
		k8sClient:  mgr.GetClient(),
		cs:         cs,
		log:        log.WithName("credential-watcher"),
		secretNS:   secretNS,
		secretName: secretName,
		tenantURL:  tenantURL,
		rateLimits: rateLimits,
		currentCfg: initialCfg,
	}

	// Use a fixed reconcile.Request key so all Secret events funnel to a
	// single reconcile call that re-reads the specific credential Secret.
	fixedRequest := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: secretNS,
			Name:      secretName,
		},
	}

	mapFn := func(_ context.Context, obj *corev1.Secret) []reconcile.Request {
		if obj.GetNamespace() == secretNS && obj.GetName() == secretName {
			return []reconcile.Request{fixedRequest}
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("credential-watcher").
		WatchesRawSource(source.Kind(
			mgr.GetCache(),
			&corev1.Secret{},
			handler.TypedEnqueueRequestsFromMapFunc(mapFn),
		)).
		Complete(watcher)
}
