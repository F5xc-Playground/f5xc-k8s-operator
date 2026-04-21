package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
)

type OriginPoolReconciler struct {
	client.Client
	Log       logr.Logger
	ClientSet *xcclientset.ClientSet
}

// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=xc.f5.com,resources=originpools/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch

func (r *OriginPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("originpool", req.NamespacedName)

	var cr v1alpha1.OriginPool
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, v1alpha1.FinalizerXCCleanup) {
		controllerutil.AddFinalizer(&cr, v1alpha1.FinalizerXCCleanup)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Resolve discover origins
	var resolved []*ResolvedOrigin
	if hasDiscoverOrigins(&cr) {
		var discovered []v1alpha1.DiscoveredOrigin
		var allResolved bool
		resolved, discovered, allResolved = r.resolveAllOrigins(ctx, &cr)
		cr.Status.DiscoveredOrigins = discovered

		if !allResolved {
			r.setCondition(&cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonDiscoveryPending, "one or more discover origins are pending")
			cr.Status.ObservedGeneration = cr.Generation
			if err := r.Status().Update(ctx, &cr); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	xcNS := cr.Spec.XCNamespace
	xc := r.ClientSet.Get()

	current, err := xc.GetOriginPool(ctx, xcNS, cr.Name)
	if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
		return r.handleXCError(ctx, log, &cr, err, "get")
	}

	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS, resolved)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current, resolved)
}

func (r *OriginPoolReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string, resolved []*ResolvedOrigin) (ctrl.Result, error) {
	create := buildOriginPoolCreate(cr, xcNS, resolved)
	result, err := xc.CreateOriginPool(ctx, xcNS, create)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "create")
	}

	log.Info("created XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonCreateSucceeded, "XC origin pool created", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string, current *xcclient.OriginPool, resolved []*ResolvedOrigin) (ctrl.Result, error) {
	desiredJSON, err := buildOriginPoolDesiredSpecJSON(cr, xcNS, resolved)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired spec JSON: %w", err)
	}

	needsUpdate, err := xc.ClientNeedsUpdate(current.RawSpec, desiredJSON)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("comparing specs: %w", err)
	}

	if !needsUpdate {
		r.setStatus(cr, true, true, v1alpha1.ReasonUpToDate, "XC origin pool is up to date", current)
		if err := r.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	replace := buildOriginPoolReplace(cr, xcNS, current.Metadata.ResourceVersion, resolved)
	result, err := xc.ReplaceOriginPool(ctx, xcNS, cr.Name, replace)
	if err != nil {
		return r.handleXCError(ctx, log, cr, err, "update")
	}

	log.Info("updated XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	r.setStatus(cr, true, true, v1alpha1.ReasonUpdateSucceeded, "XC origin pool updated", result)
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleDeletion(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, v1alpha1.FinalizerXCCleanup) {
		return ctrl.Result{}, nil
	}

	policy := cr.Annotations[v1alpha1.AnnotationDeletionPolicy]
	if policy != v1alpha1.DeletionPolicyOrphan {
		xcNS := cr.Spec.XCNamespace
		xc := r.ClientSet.Get()
		err := xc.DeleteOriginPool(ctx, xcNS, cr.Name)
		if err != nil && !errors.Is(err, xcclient.ErrNotFound) {
			return r.handleXCError(ctx, log, cr, err, "delete")
		}
		log.Info("deleted XC origin pool", "name", cr.Name, "xcNamespace", xcNS)
	} else {
		log.Info("orphaning XC origin pool", "name", cr.Name)
	}

	controllerutil.RemoveFinalizer(cr, v1alpha1.FinalizerXCCleanup)
	if err := r.Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *OriginPoolReconciler) handleXCError(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, err error, operation string) (ctrl.Result, error) {
	switch {
	case errors.Is(err, xcclient.ErrAuth):
		log.Error(err, "authentication failure — not retrying", "operation", operation)
		r.setCondition(cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, v1alpha1.ReasonAuthFailure, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{}, nil

	case errors.Is(err, xcclient.ErrConflict):
		log.Info("conflict on XC API, requeueing immediately", "operation", operation)
		reason := v1alpha1.ReasonConflict
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{Requeue: true}, nil

	case errors.Is(err, xcclient.ErrRateLimited):
		log.Info("rate limited by XC API, requeueing", "operation", operation)
		reason := v1alpha1.ReasonRateLimited
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil

	case errors.Is(err, xcclient.ErrServerError):
		log.Error(err, "XC API server error, requeueing", "operation", operation)
		reason := v1alpha1.ReasonServerError
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, reason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	default:
		log.Error(err, "XC API error, requeueing", "operation", operation)
		failReason := operationFailReason(operation)
		r.setCondition(cr, v1alpha1.ConditionSynced, metav1.ConditionFalse, failReason, err.Error())
		cr.Status.ObservedGeneration = cr.Generation
		if statusErr := r.Status().Update(ctx, cr); statusErr != nil {
			log.V(1).Error(statusErr, "failed to update status after XC error")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (r *OriginPoolReconciler) setStatus(cr *v1alpha1.OriginPool, ready, synced bool, reason, message string, xcObj *xcclient.OriginPool) {
	readyStatus := metav1.ConditionTrue
	if !ready {
		readyStatus = metav1.ConditionFalse
	}
	syncedStatus := metav1.ConditionTrue
	if !synced {
		syncedStatus = metav1.ConditionFalse
	}

	r.setCondition(cr, v1alpha1.ConditionReady, readyStatus, reason, message)
	r.setCondition(cr, v1alpha1.ConditionSynced, syncedStatus, reason, message)
	cr.Status.ObservedGeneration = cr.Generation

	if xcObj != nil {
		cr.Status.XCResourceVersion = xcObj.Metadata.ResourceVersion
		cr.Status.XCUID = xcObj.SystemMetadata.UID
		cr.Status.XCNamespace = xcObj.Metadata.Namespace
	}
}

func (r *OriginPoolReconciler) setCondition(cr *v1alpha1.OriginPool, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cr.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func operationFailReason(op string) string {
	switch op {
	case "create":
		return v1alpha1.ReasonCreateFailed
	case "update":
		return v1alpha1.ReasonUpdateFailed
	case "delete":
		return v1alpha1.ReasonDeleteFailed
	default:
		return v1alpha1.ReasonCreateFailed
	}
}

func hasDiscoverOrigins(cr *v1alpha1.OriginPool) bool {
	for _, os := range cr.Spec.OriginServers {
		if os.Discover != nil {
			return true
		}
	}
	return false
}

func (r *OriginPoolReconciler) resolveAllOrigins(ctx context.Context, cr *v1alpha1.OriginPool) ([]*ResolvedOrigin, []v1alpha1.DiscoveredOrigin, bool) {
	resolved := make([]*ResolvedOrigin, len(cr.Spec.OriginServers))
	var discovered []v1alpha1.DiscoveredOrigin
	allResolved := true

	for i, os := range cr.Spec.OriginServers {
		if os.Discover == nil {
			continue
		}

		ref := os.Discover.Resource
		ns := ref.Namespace
		if ns == "" {
			ns = cr.Namespace
		}

		raw := r.resolveResource(ctx, ref.Kind, ns, ref.Name)
		final := ResolveDiscover(os.Discover, raw)

		do := v1alpha1.DiscoveredOrigin{
			Resource: v1alpha1.ResourceRef{Kind: ref.Kind, Name: ref.Name, Namespace: ns},
			Status:   v1alpha1.DiscoveryStatusResolved,
		}

		if final.Pending {
			do.Status = v1alpha1.DiscoveryStatusPending
			do.Message = final.Message
			allResolved = false
		} else {
			do.Address = final.Address
			do.Port = final.Port
			do.AddressType = final.AddressType
			resolved[i] = &final
		}

		discovered = append(discovered, do)
	}

	return resolved, discovered, allResolved
}

func (r *OriginPoolReconciler) resolveResource(ctx context.Context, kind, ns, name string) ResolvedOrigin {
	key := client.ObjectKey{Namespace: ns, Name: name}

	switch kind {
	case "Service":
		var svc corev1.Service
		if err := r.Get(ctx, key, &svc); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Service %s/%s not found: %v", ns, name, err)}
		}
		var nodes corev1.NodeList
		if svc.Spec.Type == corev1.ServiceTypeNodePort {
			if err := r.List(ctx, &nodes); err != nil {
				return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("failed to list nodes: %v", err)}
			}
		}
		return ResolveService(&svc, nodes.Items)

	case "Ingress":
		var ing networkingv1.Ingress
		if err := r.Get(ctx, key, &ing); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Ingress %s/%s not found: %v", ns, name, err)}
		}
		return ResolveIngress(&ing)

	case "Gateway":
		var gw gatewayv1.Gateway
		if err := r.Get(ctx, key, &gw); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Gateway %s/%s not found: %v", ns, name, err)}
		}
		return ResolveGateway(&gw)

	case "Route":
		var route routev1.Route
		if err := r.Get(ctx, key, &route); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Route %s/%s not found: %v", ns, name, err)}
		}
		return ResolveRoute(&route)

	default:
		return UnsupportedKindError(kind)
	}
}

const discoverIndexKey = "spec.originServers.discover"

func discoverIndexFunc(obj client.Object) []string {
	cr, ok := obj.(*v1alpha1.OriginPool)
	if !ok {
		return nil
	}
	var refs []string
	for _, os := range cr.Spec.OriginServers {
		if os.Discover != nil {
			ref := os.Discover.Resource
			ns := ref.Namespace
			if ns == "" {
				ns = cr.Namespace
			}
			refs = append(refs, fmt.Sprintf("%s/%s/%s", ref.Kind, ns, ref.Name))
		}
	}
	return refs
}

func (r *OriginPoolReconciler) mapServiceToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Service", obj)
}

func (r *OriginPoolReconciler) mapIngressToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Ingress", obj)
}

func (r *OriginPoolReconciler) mapGatewayToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Gateway", obj)
}

func (r *OriginPoolReconciler) mapRouteToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Route", obj)
}

func (r *OriginPoolReconciler) mapNodeToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	var pools v1alpha1.OriginPoolList
	if err := r.List(ctx, &pools); err != nil {
		return nil
	}
	var requests []ctrl.Request
	seen := make(map[types.NamespacedName]bool)
	for _, pool := range pools.Items {
		for _, os := range pool.Spec.OriginServers {
			if os.Discover != nil && os.Discover.Resource.Kind == "Service" {
				key := types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace}
				if !seen[key] {
					requests = append(requests, ctrl.Request{NamespacedName: key})
					seen[key] = true
				}
			}
		}
	}
	return requests
}

func (r *OriginPoolReconciler) mapResourceToOriginPools(ctx context.Context, kind string, obj client.Object) []ctrl.Request {
	indexKey := fmt.Sprintf("%s/%s/%s", kind, obj.GetNamespace(), obj.GetName())
	var pools v1alpha1.OriginPoolList
	if err := r.List(ctx, &pools, client.MatchingFields{discoverIndexKey: indexKey}); err != nil {
		return nil
	}
	requests := make([]ctrl.Request, len(pools.Items))
	for i, pool := range pools.Items {
		requests[i] = ctrl.Request{
			NamespacedName: types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace},
		}
	}
	return requests
}

func crdInstalled(mgr ctrl.Manager, group, kind string) bool {
	_, err := mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: group, Kind: kind})
	return err == nil
}

func (r *OriginPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.OriginPool{}, discoverIndexKey, discoverIndexFunc); err != nil {
		return fmt.Errorf("indexing discover references: %w", err)
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.OriginPool{}).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.mapServiceToOriginPools)).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.mapNodeToOriginPools)).
		Watches(&networkingv1.Ingress{}, handler.EnqueueRequestsFromMapFunc(r.mapIngressToOriginPools))

	if crdInstalled(mgr, "gateway.networking.k8s.io", "Gateway") {
		builder = builder.Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayToOriginPools))
	}
	if crdInstalled(mgr, "route.openshift.io", "Route") {
		builder = builder.Watches(&routev1.Route{}, handler.EnqueueRequestsFromMapFunc(r.mapRouteToOriginPools))
	}

	return builder.Complete(r)
}
